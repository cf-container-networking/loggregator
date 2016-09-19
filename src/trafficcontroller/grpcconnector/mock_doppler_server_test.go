package grpcconnector_test

import "plumbing"

type mockDopplerServer struct {
	streamInputStreamRequest chan *plumbing.StreamRequest
	streamInputStreamServer  chan plumbing.Doppler_StreamServer
	streamOutput             chan error

	streamInputFirehoseRequest chan *plumbing.FirehoseRequest
	streamInputFirehoseServer  chan plumbing.Doppler_FirehoseServer
	firehoseOutput             chan error
}

func newMockDopplerServer() *mockDopplerServer {
	return &mockDopplerServer{
		streamInputStreamRequest: make(chan *plumbing.StreamRequest, 100),
		streamInputStreamServer:  make(chan plumbing.Doppler_StreamServer, 100),
		streamOutput:             make(chan error, 100),

		streamInputFirehoseRequest: make(chan *plumbing.FirehoseRequest, 100),
		streamInputFirehoseServer:  make(chan plumbing.Doppler_FirehoseServer, 100),
		firehoseOutput:             make(chan error, 100),
	}
}

func (m *mockDopplerServer) Stream(request *plumbing.StreamRequest, server plumbing.Doppler_StreamServer) error {
	m.streamInputStreamRequest <- request
	m.streamInputStreamServer <- server
	return <-m.streamOutput
}

func (m *mockDopplerServer) Firehose(request *plumbing.FirehoseRequest, server plumbing.Doppler_FirehoseServer) error {
	m.streamInputFirehoseRequest <- request
	m.streamInputFirehoseServer <- server
	return <-m.firehoseOutput
}
