// SPDX-License-Identifier: MIT
//
// flagctl — emergency CLI for distributed-flagd model routes.
//
// Usage:
//
//	flagctl kill-switch <flag-name> --model <model-id> [--reason <text>]
//	flagctl get <flag-name>
//	flagctl list
//
// Environment variables:
//
//	ETCD_ENDPOINT   etcd address (default: localhost:2379)
//	KAFKA_BROKERS   comma-separated Kafka brokers (default: localhost:9092)
//	FLAGCTL_USER    identity recorded in audit log (default: $USER)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/akshantvats/distributed-flagd/internal/audit"
	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	etcdEndpoint := getEnv("ETCD_ENDPOINT", "localhost:2379")
	kafkaBrokers := strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")
	caller := getEnv("FLAGCTL_USER", os.Getenv("USER"))
	if caller == "" {
		caller = "flagctl"
	}

	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("etcd connect: %v", err)
	}
	defer etcdClient.Close()

	store := etcdstore.NewClient(etcdClient)
	auditEtcd := audit.New(etcdClient)

	var kafkaProducer *audit.KafkaProducer
	kafkaProducer, err = audit.NewKafkaProducer(kafkaBrokers)
	if err != nil {
		log.Printf("warn: kafka unavailable (%v) — audit will etcd-only", err)
	} else {
		defer kafkaProducer.Close()
	}

	switch os.Args[1] {
	case "kill-switch":
		runKillSwitch(ctx, store, auditEtcd, kafkaProducer, caller)
	case "get":
		runGet(ctx, store)
	case "list":
		runList(ctx, store)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

// runKillSwitch redirects all traffic for a flag to a single model at 100%.
// It records the change in both etcd audit log and the flag_audit Kafka topic.
// If Kafka is unreachable, the kill-switch still applies — etcd is authoritative.
func runKillSwitch(ctx context.Context, store *etcdstore.Client, auditEtcd *audit.Logger, kafkaProducer *audit.KafkaProducer, caller string) {
	args := os.Args[2:]
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "kill-switch requires <flag-name>")
		os.Exit(1)
	}
	flagName := args[0]

	var modelID, reason string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--model":
			if i+1 < len(args) {
				modelID = args[i+1]
				i++
			}
		case "--reason":
			if i+1 < len(args) {
				reason = args[i+1]
				i++
			}
		}
	}
	if modelID == "" {
		fmt.Fprintln(os.Stderr, "--model <model-id> is required for kill-switch")
		os.Exit(1)
	}
	if reason == "" {
		reason = "emergency kill-switch via flagctl"
	}

	// Read current value for audit diff.
	var oldValueJSON string
	existing, err := store.GetFlag(ctx, flagName)
	if err == nil {
		b, _ := json.Marshal(existing)
		oldValueJSON = string(b)
	}

	// Kill-switch: 100% weight on the specified model, all other variants cleared.
	fd := &etcdstore.FlagData{
		Name:    flagName,
		Value:   modelID,
		Enabled: true,
		Variants: []etcdstore.VariantData{
			{Value: modelID, Weight: 100},
		},
	}
	if err := store.SetFlag(ctx, fd); err != nil {
		log.Fatalf("set kill-switch flag: %v", err)
	}
	newB, _ := json.Marshal(fd)
	newValueJSON := string(newB)

	// Write to etcd audit log.
	_ = auditEtcd.Log(ctx, audit.Entry{
		FlagName:  flagName,
		OldValue:  oldValueJSON,
		NewValue:  newValueJSON,
		ChangedBy: caller,
	})

	// Write to Kafka flag_audit topic (append-only, best-effort).
	if kafkaProducer != nil {
		kafkaEntry := audit.KafkaEntry{
			FlagName:  flagName,
			OldValue:  oldValueJSON,
			NewValue:  newValueJSON,
			ChangedBy: caller,
			Reason:    reason,
		}
		if pubErr := kafkaProducer.Publish(ctx, kafkaEntry); pubErr != nil {
			// Non-fatal: etcd is authoritative; Kafka is for observability.
			log.Printf("warn: kafka audit publish failed: %v", pubErr)
		}
	}

	fmt.Printf("kill-switch applied\n  flag:  %s\n  model: %s\n  by:    %s\n  note:  %s\n", flagName, modelID, caller, reason)
}

// runGet prints the current flag value as JSON.
func runGet(ctx context.Context, store *etcdstore.Client) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "get requires <flag-name>")
		os.Exit(1)
	}
	fd, err := store.GetFlag(ctx, os.Args[2])
	if err != nil {
		log.Fatalf("get: %v", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(fd)
}

// runList prints all flags as a JSON array.
func runList(ctx context.Context, store *etcdstore.Client) {
	flags, err := store.ListFlags(ctx)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(flags)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func usage() {
	fmt.Fprintln(os.Stderr, `flagctl — emergency CLI for distributed-flagd model routes

Commands:
  kill-switch <flag-name> --model <model-id> [--reason <text>]
      Route 100% traffic to <model-id>. Recorded in etcd + Kafka flag_audit.

  get <flag-name>
      Print current flag value as JSON.

  list
      Print all flags as a JSON array.

Env vars:
  ETCD_ENDPOINT   (default: localhost:2379)
  KAFKA_BROKERS   comma-separated brokers (default: localhost:9092)
  FLAGCTL_USER    identity in audit log (default: $USER)`)
}
