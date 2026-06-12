// SPDX-License-Identifier: MIT
//
// Requires gen/ from `make proto`. See proto/flagd.proto.
package server

import (
	"context"
	"encoding/json"
	"fmt"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/akshantvats/distributed-flagd/gen/flagd/v1"
	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
	"github.com/akshantvats/distributed-flagd/internal/eval"
)

// Server implements pb.FlagServiceServer using a shared fan-out registry
// so a single etcd watcher feeds all open EvaluateStream connections.
type Server struct {
	pb.UnimplementedFlagServiceServer
	ec       *etcdstore.Client
	registry *registry
}

// New constructs a Server and starts a background watcher that broadcasts
// flag mutations to all open EvaluateStream clients.
func New(ctx context.Context, c *clientv3.Client) *Server {
	s := &Server{
		ec:       etcdstore.NewClient(c),
		registry: newRegistry(),
	}
	go s.watchLoop(ctx)
	return s
}

// Register wires the Server into a gRPC server.
func (s *Server) Register(gs *grpc.Server) {
	pb.RegisterFlagServiceServer(gs, s)
}

// watchLoop runs for the lifetime of ctx, reading from the etcd watch channel
// and broadcasting every change to all registered stream clients.
func (s *Server) watchLoop(ctx context.Context) {
	watchChan := s.ec.WatchFlags(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-watchChan:
			if !ok {
				return
			}
			for _, ev := range resp.Events {
				var fd etcdstore.FlagData
				if err := json.Unmarshal(ev.Kv.Value, &fd); err != nil {
					continue
				}
				s.registry.broadcast(&fd)
			}
		}
	}
}

func (s *Server) GetFlag(ctx context.Context, req *pb.GetFlagRequest) (*pb.FlagValue, error) {
	fd, err := s.ec.GetFlag(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "flag %s not found", req.Name)
	}
	if !fd.Enabled {
		return &pb.FlagValue{Name: fd.Name, Value: fd.Value, Enabled: false}, nil
	}
	if len(fd.Variants) > 0 && req.HashKey != "" {
		variants := make([]eval.PercentageVariant, len(fd.Variants))
		for i, v := range fd.Variants {
			variants[i] = eval.PercentageVariant{Value: v.Value, Weight: v.Weight}
		}
		value := eval.EvaluatePercentage(req.Name, req.HashKey, variants)
		return &pb.FlagValue{Name: fd.Name, Value: value, Enabled: true, Variant: value}, nil
	}
	return &pb.FlagValue{Name: fd.Name, Value: fd.Value, Enabled: true}, nil
}

func (s *Server) SetFlag(ctx context.Context, req *pb.SetFlagRequest) (*pb.SetFlagResponse, error) {
	variants := make([]etcdstore.VariantData, len(req.Variants))
	for i, v := range req.Variants {
		variants[i] = etcdstore.VariantData{Value: v.Value, Weight: int(v.Weight)}
	}
	fd := &etcdstore.FlagData{
		Name:     req.Name,
		Value:    req.Value,
		Enabled:  req.Enabled,
		Variants: variants,
	}
	if err := s.ec.SetFlag(ctx, fd); err != nil {
		return nil, status.Errorf(codes.Internal, "set flag: %v", err)
	}
	return &pb.SetFlagResponse{Ok: true}, nil
}

// EvaluateStream sends a SNAPSHOT of all flags on connect, then streams
// DELTA updates via the shared registry fan-out instead of a per-stream watcher.
func (s *Server) EvaluateStream(req *pb.EvaluateStreamRequest, stream pb.FlagService_EvaluateStreamServer) error {
	ctx := stream.Context()

	flags, err := s.ec.ListFlags(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "list flags: %v", err)
	}
	for _, fd := range flags {
		if err := stream.Send(&pb.FlagUpdate{
			Type: pb.FlagUpdate_SNAPSHOT,
			Flag: &pb.FlagValue{Name: fd.Name, Value: fd.Value, Enabled: fd.Enabled},
		}); err != nil {
			return err
		}
	}

	streamID := fmt.Sprintf("%s/%p", req.HashKey, stream)
	ch := s.registry.subscribe(streamID)
	defer s.registry.unsubscribe(streamID)

	for {
		select {
		case <-ctx.Done():
			return nil
		case fd, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(&pb.FlagUpdate{
				Type: pb.FlagUpdate_DELTA,
				Flag: &pb.FlagValue{Name: fd.Name, Value: fd.Value, Enabled: fd.Enabled},
			}); err != nil {
				return err
			}
		}
	}
}

func (s *Server) ListFlags(ctx context.Context, _ *pb.ListFlagsRequest) (*pb.FlagList, error) {
	flags, err := s.ec.ListFlags(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list flags: %v", err)
	}
	out := make([]*pb.FlagValue, len(flags))
	for i, fd := range flags {
		out[i] = &pb.FlagValue{Name: fd.Name, Value: fd.Value, Enabled: fd.Enabled}
	}
	return &pb.FlagList{Flags: out}, nil
}
