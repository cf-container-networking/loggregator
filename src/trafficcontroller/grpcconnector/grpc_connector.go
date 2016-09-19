package grpcconnector

import (
	"context"
	"plumbing"

	"google.golang.org/grpc"
)

type Finder interface {
	GrpcURIs() []string
}

type GrpcConnector struct {
	finder      Finder
	grpcClients map[string]*grpcConnInfo
}

type grpcConnInfo struct {
	dopplerClient plumbing.DopplerClient
	conn          *grpc.ClientConn
}

func New(finder Finder) *GrpcConnector {
	return &GrpcConnector{
		finder:      finder,
		grpcClients: make(map[string]*grpcConnInfo),
	}
}

func (g *GrpcConnector) Stream(ctx context.Context, in *plumbing.StreamRequest, opts ...grpc.CallOption) (plumbing.Doppler_StreamClient, error) {
	return nil, nil
}

func (g *GrpcConnector) Firehose(ctx context.Context, in *plumbing.FirehoseRequest, opts ...grpc.CallOption) (plumbing.Doppler_FirehoseClient, error) {
	return nil, nil
}
