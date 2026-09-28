package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/MegaGone/k8s-mcp-explorer/internal/app"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewServer(svc *app.TriageService) *server.MCPServer {
	s := server.NewMCPServer("k8s-mcp-explorer", "0.1.0")

	listPods := mcp.NewTool("list_pods",
		mcp.WithDescription("List pods for N1 triage, optionally filtered by namespace."),
		mcp.WithString("namespace",
			mcp.Description("Namespace to filter by. Empty means all namespaces."),
		),
	)

	s.AddTool(listPods, listPodsHandler(svc))
	return s
}

func listPodsHandler(svc *app.TriageService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		namespace := req.GetString("namespace", "")

		pods, err := svc.ListPods(ctx, namespace)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to list pods", err), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d pod(s):\n", len(pods))
		for _, p := range pods {
			fmt.Fprintf(&b, "- %s/%s  status=%s  restarts=%d\n",
				p.Namespace, p.Name, p.Status, p.Restarts)
		}
		return mcp.NewToolResultText(b.String()), nil
	}
}

func Serve(s *server.MCPServer) error {
	return server.ServeStdio(s)
}
