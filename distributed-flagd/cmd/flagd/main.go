// SPDX-License-Identifier: MIT
//
// Run `make proto` to generate gen/flagd/v1/*.go before building.
package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"

	"github.com/akshantvats/distributed-flagd/internal/server"
)

func main() {
	etcdEndpoints := []string{getEnv("ETCD_ENDPOINT", "localhost:2379")}
	listenAddr := getEnv("LISTEN_ADDR", ":50051")

	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints: etcdEndpoints,
	})
	if err != nil {
		log.Fatalf("etcd connect: %v", err)
	}
	defer etcdClient.Close()

	svc := server.New(etcdClient)
	gs := grpc.NewServer()
	svc.Register(gs)

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("flagd listening on %s", listenAddr)
		if err := gs.Serve(ln); err != nil {
			log.Printf("serve error: %v", err)
		}
	}()

	<-ctx.Done()
	gs.GracefulStop()
	log.Println("flagd shutdown complete")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
