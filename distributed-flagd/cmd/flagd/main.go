// SPDX-License-Identifier: MIT
//
// Run `make proto` to generate gen/flagd/v1/*.go before building.
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"

	"github.com/akshantvats/distributed-flagd/internal/audit"
	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
	"github.com/akshantvats/distributed-flagd/internal/eval"
	httpapi "github.com/akshantvats/distributed-flagd/internal/http"
	"github.com/akshantvats/distributed-flagd/internal/server"
)

func main() {
	etcdEndpoints := []string{getEnv("ETCD_ENDPOINT", "localhost:2379")}
	grpcAddr := getEnv("GRPC_ADDR", ":50051")
	httpAddr := getEnv("HTTP_ADDR", ":8080")

	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdEndpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("etcd connect: %v", err)
	}
	defer etcdClient.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// gRPC server
	svc := server.New(ctx, etcdClient)
	gs := grpc.NewServer()
	svc.Register(gs)
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}
	go func() {
		log.Printf("flagd gRPC listening on %s", grpcAddr)
		if err := gs.Serve(grpcLis); err != nil {
			log.Printf("grpc serve error: %v", err)
		}
	}()

	// HTTP server
	store := etcdstore.NewClient(etcdClient)
	auditLogger := audit.New(etcdClient)
	defaultModel := getEnv("DEFAULT_MODEL_ID", "gpt-3.5-turbo")
	modelEvaluator := eval.NewModelEvaluator(store, defaultModel)
	handler := httpapi.New(store, auditLogger, modelEvaluator)
	mux := http.NewServeMux()
	httpapi.RegisterRoutes(mux, handler)
	httpServer := &http.Server{Addr: httpAddr, Handler: mux}
	go func() {
		log.Printf("flagd HTTP listening on %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http serve error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("flagd shutting down...")
	gs.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	log.Println("flagd shutdown complete")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
