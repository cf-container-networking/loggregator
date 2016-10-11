package sinkmanager

import (
	"diodes"
	"plumbing"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
)

type grpcRegistry struct {
	mu       sync.RWMutex
	registry map[string][]*bufferedGRPCSender
}

func newGRPCRegistry() *grpcRegistry {
	return &grpcRegistry{
		registry: make(map[string][]*bufferedGRPCSender),
	}
}

type GRPCSender interface {
	Send(resp *plumbing.Response) (err error)
}

func (m *SinkManager) Stream(req *plumbing.StreamRequest, d plumbing.Doppler_StreamServer) error {
	stop := make(chan struct{})
	m.RegisterStream(req, d, stop)
	defer m.UnregisterStream(req, d)
	select {
	case <-d.Context().Done():
	case <-stop:
	}
	return d.Context().Err()
}

func (m *SinkManager) RegisterStream(req *plumbing.StreamRequest, sender GRPCSender, stop chan<- struct{}) {
	m.grpcStreams.mu.Lock()
	defer m.grpcStreams.mu.Unlock()
	buffSender := newBufferedGRPCSender(sender, stop)
	m.grpcStreams.registry[req.AppID] = append(m.grpcStreams.registry[req.AppID], buffSender)
}

func (m *SinkManager) UnregisterStream(req *plumbing.StreamRequest, sender GRPCSender) {
	m.grpcStreams.mu.Lock()
	defer m.grpcStreams.mu.Unlock()

	streams := m.grpcStreams.registry[req.AppID]
	for i, buf := range streams {
		if buf.sender == sender {
			buf.Stop()
			m.grpcStreams.registry[req.AppID] = append(streams[:i], streams[i+1:]...)
			if len(m.grpcStreams.registry[req.AppID]) == 0 {
				delete(m.grpcStreams.registry, req.AppID)
			}
			return
		}
	}
}

func (m *SinkManager) Firehose(req *plumbing.FirehoseRequest, d plumbing.Doppler_FirehoseServer) error {
	stop := make(chan struct{})
	m.RegisterFirehose(req, d, stop)
	defer m.UnregisterFirehose(req)
	select {
	case <-d.Context().Done():
	case <-stop:
	}
	return d.Context().Err()
}

func (m *SinkManager) RegisterFirehose(req *plumbing.FirehoseRequest, sender GRPCSender, stop chan<- struct{}) {
	m.grpcFirehoses.mu.Lock()
	defer m.grpcFirehoses.mu.Unlock()
	buffSender := newBufferedGRPCSender(sender, stop)
	m.grpcFirehoses.registry[req.SubID] = append(m.grpcFirehoses.registry[req.SubID], buffSender)
}

func (m *SinkManager) UnregisterFirehose(req *plumbing.FirehoseRequest) {
	m.grpcFirehoses.mu.Lock()
	defer m.grpcFirehoses.mu.Unlock()

	for _, sender := range m.grpcFirehoses.registry[req.SubID] {
		sender.Stop()
	}
	delete(m.grpcFirehoses.registry, req.SubID)
}

type bufferedGRPCSender struct {
	sender GRPCSender
	diode  *diodes.OneToOne
	done   uint32
	stop   chan<- struct{}
}

func newBufferedGRPCSender(sender GRPCSender, stop chan<- struct{}) *bufferedGRPCSender {
	s := &bufferedGRPCSender{
		sender: sender,
		stop:   stop,
	}

	s.diode = diodes.NewOneToOne(1000, s)
	go s.run()

	return s
}

func (s *bufferedGRPCSender) run() {
	for {
		done := atomic.LoadUint32(&s.done)
		if done > 0 {
			return
		}
		payload, ok := s.diode.TryNext()
		if ok {
			err := s.sender.Send(&plumbing.Response{
				Payload: payload,
			})
			if err != nil {
				close(s.stop)
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (s *bufferedGRPCSender) Send(resp *plumbing.Response) (err error) {
	s.diode.Set(resp.Payload)
	return nil
}

func (s *bufferedGRPCSender) Stop() {
	atomic.AddUint32(&s.done, 1)
}

func (s *bufferedGRPCSender) Alert(missed int) {
	metrics.BatchAddCounter("Diode.totalDroppedMessages", uint64(missed))
}
