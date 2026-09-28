# K8s.Mcp.Explorer — Planning Document

**Project:** MCP server for k3s N1 support agent
**Status:** Planning
**Owner:** jmartinez

---

## 1. Objective

Build an MCP (Model Context Protocol) server that exposes read-only k3s
operations and a runbook knowledge layer, so an AI agent can perform **N1
support triage**: gather evidence, review logs, inspect flows, and match
reported symptoms against curated runbooks.

The agent **does not act on the cluster** in phase 1 — it observes, diagnoses,
and hands off to a human.

---

## 2. Architecture (layered, decoupled)

```
┌──────────────┐   ┌─────────────┐   ┌──────────────┐   ┌──────┐
│  SURFACE     │──►│   AGENT     │──►│  MCP SERVER  │──►│ k3s  │
│ Teams / Jira │   │  (LLM)      │   │ k3s-n1-mcp   │   │ API  │
└──────────────┘   └─────────────┘   └──────────────┘   └──────┘
  where humans        who reasons       the tooling      the cluster
  interact
```

Four independent, swappable pieces:

| Piece            | Role                              | Deployment                    |
| ---------------- | --------------------------------- | ----------------------------- |
| **MCP k3s**      | Tooling (read-only k3s + runbooks)| `Deployment` inside k3s       |
| **Agent (LLM)**  | Reasoning                         | Consumes MCP tools            |
| **Surface**      | Human interaction channel         | Teams bot / Jira webhook glue |
| **MCP Jira**     | (Optional) read/comment tickets   | External tool for the agent   |

**Key principle:** the surface is decoupled from the tooling. The same agent +
MCP can be exposed through Teams, Slack, or Jira by swapping only the adapter
("glue") layer — the MCP is never touched.

---

## 3. Implementation decisions (locked)

| Decision              | Choice                        | Rationale                                                                 |
| --------------------- | ----------------------------- | ------------------------------------------------------------------------- |
| **Scope**             | Read-only now, actions later  | Agent is probabilistic; a hallucinating reader is harmless, a writer isn't |
| **Language**          | Go (`client-go`)              | Native k8s ecosystem, single binary, strong typing against the k8s API    |
| **Deployment**        | In-cluster `Deployment`       | Uses in-cluster config → SA identity is automatic, no external kubeconfig |
| **Security boundary** | RBAC at the cluster           | k3s denies dangerous verbs; safety lives in the cluster, not the prompt   |
| **Runbook storage**   | Versioned `.md` files (start) | Git-friendly, reviewable in PR; Engram as semantic backend later          |
| **Human-in-the-loop** | Agent triages, never resolves | Final decision stays human; agent gathers evidence + proposes             |
| **Build strategy**    | Build minimal MCP from scratch | Goal is learning; existing servers used as reference, not forked          |
| **Multi-tenancy**     | One MCP instance per client   | Physical isolation; a routing bug can't reach another client's cluster    |

**On reusing existing servers:** `containers/kubernetes-mcp-server` (Apache 2.0,
Go, client-go, multi-cluster, PII redaction) and `reza-gholizade/k8s-mcp-server`
(Go, simpler) are studied as **reference** for how to register tools and
structure client-go. They are NOT forked — the learning goal requires building
the minimal server ourselves. `reza-gholizade` is the fallback scaffold if a
starting point is needed.

---

## 3b. Multi-tenancy & tenant routing

Each client runs on its own dedicated k3s cluster. Isolation is **physical**, not
prompt-based:

- **One MCP instance per client**, each configured with ONLY that client's
  kubeconfig / ServiceAccount. Never a single instance holding all clusters.
- Each instance has its own read-only RBAC in its own cluster.

**Routing (glue layer):** deciding which client a ticket belongs to is an
**authorization decision** and lives in deterministic glue code — never in the
LLM.

- Explicit allowlist mapping (`key → client instance`), exact match.
- **Fail-closed:** unknown / ambiguous / unmapped key → reject + escalate to
  human. No silent default cluster, ever.
- **Routing key:** prefer a **structured Jira signal** (project/board or an
  explicit client field) over parsing the reporter's email domain. Email domains
  are fragile (spoofing, shared domains e.g. gmail, typos/homoglyphs,
  multi-client consultants). Email may serve as secondary cross-validation, not
  the primary key.

Defense in depth: correct routing picks the right instance; instance isolation
bounds the blast radius if routing ever fails.

---

## 4. Project structure

```
k3s-n1-mcp/
├── cmd/
│   └── server/main.go        # MCP server entrypoint (stdio)
├── internal/
│   ├── mcp/                   # tool registration, MCP handlers
│   ├── k8s/                   # client-go (read-only)
│   │   ├── client.go          # init from in-cluster / kubeconfig
│   │   ├── pods.go            # list, status, logs
│   │   ├── events.go          # namespace events
│   │   └── workloads.go       # deployments, rollouts, endpoints
│   ├── runbook/               # knowledge layer
│   │   ├── store.go           # load + search runbooks
│   │   └── runbooks/*.md      # versioned runbooks
│   └── safety/                # allowlist, namespace validation
├── deploy/
│   ├── serviceaccount.yaml    # SA + read-only ClusterRole + binding
│   └── mcp-config.json        # agent-side registration
└── go.mod
```

---

## 5. MCP tools

### Phase 1 — Observability (read-only)

| Tool                 | Purpose                                    | N1 use              |
| -------------------- | ------------------------------------------ | ------------------- |
| `list_pods`          | Pods by namespace/label, status, restarts  | "what is down?"     |
| `get_pod_logs`       | Logs with `tail`, `since`, `previous`      | primary evidence    |
| `describe_pod`       | Detailed state + conditions + last state   | why it crashes      |
| `get_events`         | Namespace events, time-ordered             | correlation         |
| `get_rollout_status` | Deployment/rollout state                   | "did it deploy?"    |
| `get_endpoints`      | Whether a Service resolves to live pods    | flow inspection     |
| `top_pods` / `top_nodes` | CPU/mem                                | OOM / saturation    |

**Design rule:** tools return **structured, bounded** output. Default log
`tail=200`; the agent asks for more only if needed. Avoid context dilution —
dumping 10k log lines drowns the model.

### Phase 2 — Actions (deferred, elevated privilege)

- `restart_deployment`, `scale`, `rollout_undo`, `delete_pod`
- Separate `ClusterRole` scoped to specific resources/namespaces (never `*`)
- Always `dry-run` first + explicit human confirmation
- Hardcoded allowlist in `internal/safety/`

---

## 6. Runbook layer

Each runbook has a fixed structure so the agent uses it reliably:

```markdown
---
id: search-crashloop
sintomas: [CrashLoopBackOff, search no responde, timeouts en /search]
componente: search
severidad: alta
---
## Causa probable
El pod de search se queda sin memoria al cargar el índice.

## Verificación
1. get_events → buscar "OOMKilled"
2. describe_pod → revisar lastState.terminated.reason
3. top_pods → confirmar límite de memoria

## Remediación
- Subir memory limit del deployment search
- (fase 2) restart_deployment search
```

Tool `search_runbook(symptom)` matches against frontmatter `sintomas` and
returns the runbook. The agent then follows the verification steps using
phase-1 tools.

**Escalation rule:** if no runbook matches, the agent does **not** invent a
cause — it labels `needs-human` and escalates with the raw evidence it could
gather. An agent that says "I don't know" is valuable; one that confabulates is
dangerous.

**Evolution:** start with `.md` files. Later, connect **Engram** as a semantic
backend so "the search is slow" matches "search timeouts". Do not start there.

---

## 7. Security (RBAC)

Read-only is **real**, enforced by the cluster:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: n1-mcp-readonly
rules:
  - apiGroups: ["", "apps"]
    resources: ["pods", "pods/log", "events", "deployments",
                "replicasets", "services", "endpoints"]
    verbs: ["get", "list", "watch"]   # no delete/patch/create
```

The MCP pod runs under this ServiceAccount. Even if the model hallucinates a
`delete`, the k3s API server rejects it. Security lives in the cluster, not the
prompt.

---

## 8. Surfaces

### Jira (reactive, ticket-driven) — primary target flow

```
Stakeholder → Ticket in Jira → webhook → Glue service → Agent
                                                           │
                                                           ├─► MCP k3s (logs, events, pods)
                                                           ├─► search_runbook
                                                           ▼
                                                    Initial triage
                                                           │
                                        comments on ticket ◄┘ (evidence + diagnosis + runbook)
```

Design considerations:

1. **Glue service** — a small HTTP service receives the Jira webhook,
   validates it, and invokes the agent. Not the MCP, not the LLM directly. Can
   run in the same k3s.
2. **Loop prevention** — the agent comments on the ticket; if the webhook
   listens to "any change" the agent's own comment re-triggers it → infinite
   loop. Filter by event (`issue_created`, not `issue_updated`) and/or ignore
   changes made by the agent's bot user.
3. **Human-in-the-loop** — the agent triages and proposes; it does not close
   the ticket or touch the cluster. Posts evidence + diagnosis, labels
   `triaged-by-agent` / `needs-human`.
4. **Sanitization** — cluster logs may contain PII/secrets. A Jira comment is
   permanent and visible. Redact before posting.

### Teams (conversational) — optional

Agent behind a Teams bot (Azure Bot Framework). Human chats → bot → agent →
MCP → reply in chat. The agent doesn't know it's Teams; the bot is an I/O
adapter.

---

## 9. Risks

| Risk                          | Mitigation                                              |
| ----------------------------- | ------------------------------------------------------- |
| **Cross-tenant data leak**    | One MCP instance per client; deterministic fail-closed routing in glue; never LLM-decided |
| Agent hallucinates diagnosis  | Read-only RBAC; human-in-the-loop; escalate on no-match |
| Webhook comment loop          | Filter by event type / ignore bot-authored changes      |
| PII/secrets leaked to Jira    | Sanitize log output before commenting                   |
| Context dilution (huge logs)  | Bounded tool output, sensible defaults                  |
| Scope creep into write ops    | Phase 2 isolated behind separate elevated RBAC          |

---

## 10. Suggested build order

1. Read-only k8s client + 3 minimal tools (`list_pods`, `get_pod_logs`,
   `get_events`) → a working N1 skeleton.
2. Remaining observability tools.
3. Runbook layer (`.md` files).
4. Jira glue service + webhook flow.
5. (Later) Engram semantic runbooks / phase-2 actions / traffic capture.

---

## Out of scope (phase 1)

- **Traffic capture (ngrep/tcpdump)** — requires elevated network privileges
  (`NET_RAW`/`NET_ADMIN`), breaks the read-only model, and is blind to TLS
  traffic. Deferred to phase 2 as an isolated, human-triggered, time-boxed tool
  with its own elevated RBAC.
- **Write/remediation actions** — phase 2.
