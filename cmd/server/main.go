package main

import (
	"log"
	"os"

	"github.com/MegaGone/k8s-mcp-explorer/internal/adapters/k8s"
	mcpadapter "github.com/MegaGone/k8s-mcp-explorer/internal/adapters/mcp"
	"github.com/MegaGone/k8s-mcp-explorer/internal/app"
	"github.com/MegaGone/k8s-mcp-explorer/internal/domain"
)

func main() {
	cluster, err := selectClusterReader()
	if err != nil {
		log.Fatalf("cluster init: %v", err)
	}

	svc := app.NewTriageService(cluster)
	s := mcpadapter.NewServer(svc)

	log.Println("k8s-mcp-explorer starting on stdio...")
	if err := mcpadapter.Serve(s); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func selectClusterReader() (domain.ClusterReader, error) {
	if os.Getenv("K8S_MCP_REAL") != "1" {
		return k8s.NewFakeClusterReader(), nil
	}
	client, err := k8s.NewClientset()
	if err != nil {
		return nil, err
	}
	return k8s.NewRealClusterReader(client), nil
}
