package sinkmanager_test

import (
	"doppler/sinkserver/sinkmanager"
	"errors"
	"plumbing"
	"runtime"
	"time"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("SinkManager GRPC", func() {
	var m *sinkmanager.SinkManager

	BeforeEach(func() {
		m = sinkmanager.New(
			1,
			true,
			nil,
			loggertesthelper.Logger(),
			0,
			"origin",
			time.Second,
			time.Second,
			time.Second,
			time.Second,
		)
		fakeEventEmitter.Reset()
	})

	Describe("Stream", func() {
		It("routes messages to GRPC streams", func() {
			req := plumbing.StreamRequest{AppID: "app"}

			firstSender := newMockGRPCSender()
			close(firstSender.SendOutput.Err)
			stop1 := make(chan struct{})
			m.RegisterStream(&req, firstSender, stop1)

			secondSender := newMockGRPCSender()
			close(secondSender.SendOutput.Err)
			stop2 := make(chan struct{})
			m.RegisterStream(&req, secondSender, stop2)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}

			m.SendTo("app", env)
			Eventually(firstSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
			Eventually(secondSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
		})

		It("doesn't send to streams for different app IDs", func() {
			req := plumbing.StreamRequest{AppID: "app"}

			sender := newMockGRPCSender()
			close(sender.SendOutput.Err)
			stop := make(chan struct{})
			m.RegisterStream(&req, sender, stop)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}
			m.SendTo("another-app", env)

			Consistently(sender.SendInput).ShouldNot(BeCalled())
		})

		It("can concurrently register Streams without data races", func() {
			req := plumbing.StreamRequest{AppID: "app"}
			sender := newMockGRPCSender()
			close(sender.SendOutput.Err)

			stop1 := make(chan struct{})
			go m.RegisterStream(&req, sender, stop1)

			stop2 := make(chan struct{})
			m.RegisterStream(&req, sender, stop2)
		})

		It("continues to send while a sender is blocking", func(done Done) {
			defer close(done)

			req := plumbing.StreamRequest{AppID: "app"}

			blockingSender := newMockGRPCSender()
			stop1 := make(chan struct{})
			m.RegisterStream(&req, blockingSender, stop1)

			workingSender := newMockGRPCSender()
			close(workingSender.SendOutput.Err)
			stop2 := make(chan struct{})
			m.RegisterStream(&req, workingSender, stop2)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}
			m.SendTo("app", env)

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}

			Eventually(workingSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
		})

		It("unregisters the connection", func() {
			req := plumbing.StreamRequest{AppID: "app"}

			firstSender := newMockGRPCSender()
			firstSender.SendOutput.Err <- nil
			stop1 := make(chan struct{})
			m.RegisterStream(&req, firstSender, stop1)

			secondSender := newMockGRPCSender()
			close(secondSender.SendOutput.Err)
			stop2 := make(chan struct{})
			m.RegisterStream(&req, secondSender, stop2)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}

			m.SendTo("app", env)
			Eventually(firstSender.SendInput).Should(BeCalled(
				With(expected),
			))
			Eventually(secondSender.SendInput).Should(BeCalled(
				With(expected),
			))

			routines := runtime.NumGoroutine()

			m.UnregisterStream(&req, firstSender)
			m.SendTo("app", env)
			Consistently(firstSender.SendInput).ShouldNot(BeCalled())
			Eventually(secondSender.SendInput).Should(BeCalled(With(expected)))

			By("asserting that the connection's goroutine(s) is/are closed")
			Eventually(runtime.NumGoroutine).Should(BeNumerically("<", routines))
		})

		It("closes the stop channel when send returns an error", func() {
			req := plumbing.StreamRequest{AppID: "app"}

			sender := newMockGRPCSender()
			stop := make(chan struct{})
			m.RegisterStream(&req, sender, stop)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}
			m.SendTo("app", env)
			sender.SendOutput.Err <- errors.New("boom")

			Eventually(stop).Should(BeClosed())
		})
	})

	Describe("Firehose", func() {
		It("gets to drink from the firehose", func() {
			// see: https://www.youtube.com/watch?v=OXc5ltzKq3Y

			firstReq := plumbing.FirehoseRequest{SubID: "first-subscription"}
			firstSender := newMockGRPCSender()
			close(firstSender.SendOutput.Err)
			stop1 := make(chan struct{})
			m.RegisterFirehose(&firstReq, firstSender, stop1)

			secondReq := plumbing.FirehoseRequest{SubID: "second-subscription"}
			secondSender := newMockGRPCSender()
			close(secondSender.SendOutput.Err)
			stop2 := make(chan struct{})
			m.RegisterFirehose(&secondReq, secondSender, stop2)

			env := &events.Envelope{
				EventType: events.Envelope_ContainerMetric.Enum(),
				Origin:    proto.String("origin"),
				ContainerMetric: &events.ContainerMetric{
					ApplicationId: proto.String("app"),
					InstanceIndex: proto.Int32(1),
					CpuPercentage: proto.Float64(12.3),
					MemoryBytes:   proto.Uint64(1),
					DiskBytes:     proto.Uint64(1),
				},
			}
			m.SendTo("app", env)

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}
			Eventually(firstSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
			Eventually(secondSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
		})

		It("fans out messages to multiple senders for the same subscription", func() {
			req := plumbing.FirehoseRequest{SubID: "subscription"}

			firstSender := newMockGRPCSender()
			close(firstSender.SendOutput.Err)
			stop1 := make(chan struct{})
			m.RegisterFirehose(&req, firstSender, stop1)

			secondSender := newMockGRPCSender()
			close(secondSender.SendOutput.Err)
			stop2 := make(chan struct{})
			m.RegisterFirehose(&req, secondSender, stop2)

			env := &events.Envelope{
				EventType: events.Envelope_ContainerMetric.Enum(),
				Origin:    proto.String("origin"),
				ContainerMetric: &events.ContainerMetric{
					ApplicationId: proto.String("app"),
					InstanceIndex: proto.Int32(1),
					CpuPercentage: proto.Float64(12.3),
					MemoryBytes:   proto.Uint64(1),
					DiskBytes:     proto.Uint64(1),
				},
			}
			m.SendTo("app", env)

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}
			var received *plumbing.Response
			select {
			case received = <-firstSender.SendInput.Resp:
			case received = <-secondSender.SendInput.Resp:
			case <-time.After(time.Second):
				Fail("Timed out waiting for firehose message")
			}
			Expect(received).To(Equal(expected))
			Consistently(firstSender.SendInput).ShouldNot(BeCalled())
			Consistently(secondSender.SendInput).ShouldNot(BeCalled())
		})

		It("can concurrently register Firehoses without data races", func() {
			req := plumbing.FirehoseRequest{SubID: "subscription"}
			sender := newMockGRPCSender()
			close(sender.SendOutput.Err)

			stop1 := make(chan struct{})
			go m.RegisterFirehose(&req, sender, stop1)

			stop2 := make(chan struct{})
			m.RegisterFirehose(&req, sender, stop2)
		})

		It("continues to send while a sender is blocking", func(done Done) {
			defer close(done)

			req1 := plumbing.FirehoseRequest{SubID: "sub-1"}
			req2 := plumbing.FirehoseRequest{SubID: "sub-2"}

			blockingSender := newMockGRPCSender()
			stop1 := make(chan struct{})
			m.RegisterFirehose(&req1, blockingSender, stop1)

			workingSender := newMockGRPCSender()
			close(workingSender.SendOutput.Err)
			stop2 := make(chan struct{})
			m.RegisterFirehose(&req2, workingSender, stop2)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}
			m.SendTo("app", env)

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}

			Eventually(workingSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
		})

		It("reports the number of dropped messages", func() {
			req := plumbing.FirehoseRequest{SubID: "sub-1"}
			blockingSender := newMockGRPCSender()
			stop := make(chan struct{})
			m.RegisterFirehose(&req, blockingSender, stop)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}

			By("waiting for the diode to grab it's first entry")
			m.SendTo("app", env)
			Eventually(blockingSender.SendCalled).Should(Receive())

			By("over running the diode's ring buffer by 1")
			for i := 0; i < 1001; i++ {
				m.SendTo("app", env)
			}

			By("single successful end to invoke the alert")
			blockingSender.SendOutput.Err <- nil

			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("Diode.totalDroppedMessages", uint64(1000)),
			))
		})

		It("unregisters", func() {
			req := plumbing.FirehoseRequest{SubID: "first-subscription"}
			sender := newMockGRPCSender()
			close(sender.SendOutput.Err)

			env := &events.Envelope{
				EventType: events.Envelope_ContainerMetric.Enum(),
				Origin:    proto.String("origin"),
				ContainerMetric: &events.ContainerMetric{
					ApplicationId: proto.String("app"),
					InstanceIndex: proto.Int32(1),
					CpuPercentage: proto.Float64(12.3),
					MemoryBytes:   proto.Uint64(1),
					DiskBytes:     proto.Uint64(1),
				},
			}
			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())
			expected := &plumbing.Response{
				Payload: payload,
			}

			stop := make(chan struct{})
			m.RegisterFirehose(&req, sender, stop)
			m.SendTo("app", env)
			Eventually(sender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))

			routines := runtime.NumGoroutine()

			m.UnregisterFirehose(&req)
			m.SendTo("app", env)
			Consistently(sender.SendInput.Resp).ShouldNot(BeCalled(
				With(expected),
			))

			By("asserting that the connection's goroutine(s) is/are closed")
			Eventually(runtime.NumGoroutine).Should(BeNumerically("<", routines))
		})

		It("closes the stop channel when send returns an error", func() {
			req := plumbing.FirehoseRequest{SubID: "first-subscription"}
			sender := newMockGRPCSender()
			sender.SendOutput.Err <- errors.New("boom")

			env := &events.Envelope{
				EventType: events.Envelope_ContainerMetric.Enum(),
				Origin:    proto.String("origin"),
				ContainerMetric: &events.ContainerMetric{
					ApplicationId: proto.String("app"),
					InstanceIndex: proto.Int32(1),
					CpuPercentage: proto.Float64(12.3),
					MemoryBytes:   proto.Uint64(1),
					DiskBytes:     proto.Uint64(1),
				},
			}

			stop := make(chan struct{})
			m.RegisterFirehose(&req, sender, stop)
			m.SendTo("app", env)

			Eventually(stop).Should(BeClosed())
		})
	})
})
