package grpcmanager_test

import (
	"io"
	"net"
	"plumbing"
	"time"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"doppler/grpcmanager"

	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("GRPCManager", func() {
	var (
		mockRegistrar  *mockRegistrar
		mockCleanup    func()
		cleanupCalled  chan struct{}
		mockDataDumper *mockDataDumper

		manager       *grpcmanager.GRPCManager
		listener      net.Listener
		connCloser    io.Closer
		dopplerClient plumbing.DopplerClient

		subscribeRequest *plumbing.SubscriptionRequest

		setter grpcmanager.DataSetter
	)

	var startGRPCServer = func(ds plumbing.DopplerServer) net.Listener {
		lis, err := net.Listen("tcp", ":0")
		Expect(err).ToNot(HaveOccurred())
		s := grpc.NewServer()
		plumbing.RegisterDopplerServer(s, ds)
		go s.Serve(lis)

		return lis
	}

	var establishClient = func(dopplerAddr string) (plumbing.DopplerClient, io.Closer) {
		conn, err := grpc.Dial(dopplerAddr, grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		c := plumbing.NewDopplerClient(conn)

		return c, conn
	}

	var fetchSetter = func() grpcmanager.DataSetter {
		var s grpcmanager.DataSetter
		Eventually(mockRegistrar.RegisterInput.Setter).Should(
			Receive(&s),
		)
		return s
	}

	var buildCleanup = func(c chan struct{}) func() {
		return func() {
			close(c)
		}
	}

	BeforeEach(func() {
		mockRegistrar = newMockRegistrar()
		cleanupCalled = make(chan struct{})
		mockCleanup = buildCleanup(cleanupCalled)
		mockRegistrar.RegisterOutput.Ret0 <- mockCleanup
		mockDataDumper = newMockDataDumper()

		manager = grpcmanager.New(mockRegistrar, mockDataDumper)

		listener = startGRPCServer(manager)
		dopplerClient, connCloser = establishClient(listener.Addr().String())

		subscribeRequest = &plumbing.SubscriptionRequest{
			Filter: &plumbing.FilterRequest{
				AppID: "some-app-id",
			},
		}
	})

	AfterEach(func() {
		connCloser.Close()
		listener.Close()
	})

	Describe("registration", func() {
		It("registers subscription", func() {
			_, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
			Expect(err).ToNot(HaveOccurred())

			Eventually(mockRegistrar.RegisterInput).Should(
				BeCalled(With(subscribeRequest, Not(BeNil()))),
			)
		})

		Context("connection is established", func() {
			Context("client does not close the connection", func() {
				It("does not unregister itself", func() {
					dopplerClient.Subscribe(context.TODO(), subscribeRequest)
					fetchSetter()

					Consistently(cleanupCalled).ShouldNot(BeClosed())
				})
			})

			Context("client closes connection", func() {
				It("unregisters subscription", func() {
					dopplerClient.Subscribe(context.TODO(), subscribeRequest)
					setter = fetchSetter()
					connCloser.Close()

					Eventually(cleanupCalled).Should(BeClosed())
				})
			})
		})
	})

	Describe("data transmission", func() {
		var readFromReceiver = func(r plumbing.Doppler_SubscribeClient) <-chan []byte {
			c := make(chan []byte, 100)

			go func() {
				for {
					resp, err := r.Recv()
					if err != nil {
						return
					}

					c <- resp.Payload
				}
			}()

			return c
		}

		It("sends data from the setter to the client", func() {
			rx, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
			Expect(err).ToNot(HaveOccurred())

			setter = fetchSetter()
			setter.Set([]byte("some-data-0"))
			setter.Set([]byte("some-data-1"))
			setter.Set([]byte("some-data-2"))

			c := readFromReceiver(rx)
			Eventually(c).Should(BeCalled(With(
				[]byte("some-data-0"),
				[]byte("some-data-1"),
				[]byte("some-data-2"),
			)))
		})
	})

	Describe("container metrics", func() {
		It("returns container metrics from its data dumper", func() {
			envelope, data := buildContainerMetric()
			mockDataDumper.LatestContainerMetricsOutput.Ret0 <- []*events.Envelope{
				envelope,
			}

			resp, err := dopplerClient.ContainerMetrics(context.TODO(),
				&plumbing.ContainerMetricsRequest{AppID: "some-app"})

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Payload).To(ContainElement(
				data,
			))
			Expect(mockDataDumper.LatestContainerMetricsInput).To(BeCalled(
				With("some-app"),
			))
		})

		It("throw away invalid envelopes from its data dumper", func() {
			envelope, _ := buildContainerMetric()
			mockDataDumper.LatestContainerMetricsOutput.Ret0 <- []*events.Envelope{
				&events.Envelope{},
				envelope,
			}

			resp, err := dopplerClient.ContainerMetrics(context.TODO(),
				&plumbing.ContainerMetricsRequest{AppID: "some-app"})

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Payload).To(HaveLen(1))
		})
	})

	Describe("recent logs", func() {
		It("returns recent logs from its data dumper", func() {
			envelope, data := buildLogMessage()
			mockDataDumper.RecentLogsForOutput.Ret0 <- []*events.Envelope{
				envelope,
			}
			resp, err := dopplerClient.RecentLogs(context.TODO(),
				&plumbing.RecentLogsRequest{AppID: "some-app"})
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Payload).To(ContainElement(
				data,
			))
			Expect(mockDataDumper.RecentLogsForInput).To(BeCalled(
				With("some-app"),
			))
		})

		It("throw away invalid envelopes from its data dumper", func() {
			envelope, _ := buildLogMessage()
			mockDataDumper.RecentLogsForOutput.Ret0 <- []*events.Envelope{
				&events.Envelope{},
				envelope,
			}

			resp, err := dopplerClient.RecentLogs(context.TODO(),
				&plumbing.RecentLogsRequest{AppID: "some-app"})

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Payload).To(HaveLen(1))
		})
	})
})

func buildContainerMetric() (*events.Envelope, []byte) {
	envelope := &events.Envelope{
		Origin:    proto.String("doppler"),
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("some-app"),
			InstanceIndex: proto.Int32(int32(1)),
			CpuPercentage: proto.Float64(float64(1)),
			MemoryBytes:   proto.Uint64(uint64(1)),
			DiskBytes:     proto.Uint64(uint64(1)),
		},
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}

func buildLogMessage() (*events.Envelope, []byte) {
	envelope := &events.Envelope{
		Origin:    proto.String("doppler"),
		EventType: events.Envelope_LogMessage.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		LogMessage: &events.LogMessage{
			Message:     []byte("some-log-message"),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
		},
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}
