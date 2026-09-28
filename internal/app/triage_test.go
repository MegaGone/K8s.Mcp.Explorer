package app

import (
	"context"
	"errors"
	"testing"

	"github.com/MegaGone/k8s-mcp-explorer/internal/domain"
)

type stubReader struct {
	pods []domain.Pod
	err  error
}

func (s stubReader) ListPods(context.Context, string) ([]domain.Pod, error) {
	return s.pods, s.err
}

func TestTriageService_ListPods_ReturnsPods(t *testing.T) {
	want := []domain.Pod{{Name: "search-7d9f", Namespace: "default", Status: "Running"}}
	svc := NewTriageService(stubReader{pods: want})

	got, err := svc.ListPods(context.Background(), "default")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "search-7d9f" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestTriageService_ListPods_PropagatesError(t *testing.T) {
	wantErr := errors.New("cluster unreachable")
	svc := NewTriageService(stubReader{err: wantErr})

	_, err := svc.ListPods(context.Background(), "")

	if !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}
}
