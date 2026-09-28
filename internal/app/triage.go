package app

import (
	"context"

	"github.com/MegaGone/k8s-mcp-explorer/internal/domain"
)

type TriageService struct {
	cluster domain.ClusterReader
}

func NewTriageService(cluster domain.ClusterReader) *TriageService {
	return &TriageService{cluster: cluster}
}

func (s *TriageService) ListPods(ctx context.Context, namespace string) ([]domain.Pod, error) {
	return s.cluster.ListPods(ctx, namespace)
}
