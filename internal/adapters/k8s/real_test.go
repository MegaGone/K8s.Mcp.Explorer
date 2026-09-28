package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func seededClient() *fake.Clientset {
	return fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "search-7d9f", Namespace: "default"},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{{RestartCount: 0}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "search-indexer-2b1a", Namespace: "default"},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{{
					RestartCount: 7,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "api-gateway-55c8", Namespace: "platform"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)
}

func TestRealClusterReader_ListPods_MapsFields(t *testing.T) {
	r := NewRealClusterReader(seededClient())

	pods, err := r.ListPods(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pods) != 2 {
		t.Fatalf("got %d pods, want 2", len(pods))
	}

	byName := map[string]struct {
		status   string
		restarts int
	}{}
	for _, p := range pods {
		byName[p.Name] = struct {
			status   string
			restarts int
		}{p.Status, p.Restarts}
	}

	if got := byName["search-indexer-2b1a"]; got.status != "CrashLoopBackOff" || got.restarts != 7 {
		t.Fatalf("indexer mapped wrong: %+v", got)
	}
	if got := byName["search-7d9f"]; got.status != "Running" || got.restarts != 0 {
		t.Fatalf("running pod mapped wrong: %+v", got)
	}
}

func TestRealClusterReader_ListPods_EmptyNamespaceListsAll(t *testing.T) {
	r := NewRealClusterReader(seededClient())

	pods, err := r.ListPods(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pods) != 3 {
		t.Fatalf("got %d pods, want 3 across all namespaces", len(pods))
	}
}
