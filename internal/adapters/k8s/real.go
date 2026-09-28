package k8s

import (
	"context"

	"github.com/MegaGone/k8s-mcp-explorer/internal/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RealClusterReader struct {
	client kubernetes.Interface
}

func NewRealClusterReader(client kubernetes.Interface) *RealClusterReader {
	return &RealClusterReader{client: client}
}

func (r *RealClusterReader) ListPods(ctx context.Context, namespace string) ([]domain.Pod, error) {
	list, err := r.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]domain.Pod, 0, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		pods = append(pods, domain.Pod{
			Name:      p.Name,
			Namespace: p.Namespace,
			Status:    podStatus(p),
			Restarts:  podRestarts(p),
		})
	}
	return pods, nil
}

func podStatus(p *corev1.Pod) string {
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return cs.State.Waiting.Reason
		}
	}
	return string(p.Status.Phase)
}

func podRestarts(p *corev1.Pod) int {
	var total int
	for _, cs := range p.Status.ContainerStatuses {
		total += int(cs.RestartCount)
	}
	return total
}
