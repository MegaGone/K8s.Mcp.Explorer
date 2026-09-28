package k8s

import (
	"context"

	"github.com/MegaGone/k8s-mcp-explorer/internal/domain"
)

// TODO: replace with a real client-go adapter (in-cluster, read-only SA).
type FakeClusterReader struct{}

func NewFakeClusterReader() *FakeClusterReader {
	return &FakeClusterReader{}
}

func (f *FakeClusterReader) ListPods(_ context.Context, namespace string) ([]domain.Pod, error) {
	pods := []domain.Pod{
		{Name: "search-7d9f", Namespace: "default", Status: "Running", Restarts: 0},
		{Name: "search-indexer-2b1a", Namespace: "default", Status: "CrashLoopBackOff", Restarts: 7},
		{Name: "api-gateway-55c8", Namespace: "platform", Status: "Running", Restarts: 1},
	}

	if namespace == "" {
		return pods, nil
	}
	filtered := make([]domain.Pod, 0, len(pods))
	for _, p := range pods {
		if p.Namespace == namespace {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}
