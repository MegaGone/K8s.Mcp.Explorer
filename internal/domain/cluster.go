package domain

import "context"

type Pod struct {
	Name      string
	Namespace string
	Status    string
	Restarts  int
}

type ClusterReader interface {
	ListPods(ctx context.Context, namespace string) ([]Pod, error)
}
