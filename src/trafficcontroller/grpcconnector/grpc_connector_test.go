//go:generate hel
package grpcconnector_test

import (
	"context"
	"net"
	"plumbing"
	"trafficcontroller/grpcconnector"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GrpcConnector", func() {
	var (
		grpcListeners []net.Listener
		mockFinder    *mockFinder

		connector *grpcconnector.GrpcConnector
	)

	var createGrpcServer = func(server *mockDopplerServer) string {
		lis, err := net.Listen("tcp", ":0")
		Expect(err).ToNot(HaveOccurred())
		grpcListeners = append(grpcListeners, lis)
		s := grpc.NewServer()
		plumbing.RegisterDopplerServer(s, server)
		go s.Serve(lis)
		return lis.Addr().String()
	}

	BeforeEach(func() {
		mockFinder = newMockFinder()
		connector = grpcconnector.New(mockFinder)
	})

	AfterEach(func() {
		for _, l := range grpcListeners {
			l.Close()
		}
	})

	Describe("connections to grpc servers", func() {
		Context("when two servers are advertised", func() {
			var (
				dopplerA *mockDopplerServer
				dopplerB *mockDopplerServer
			)

			var fetchPayloads = func(count int, client plumbing.Doppler_StreamClient) chan []byte {
				c := make(chan []byte, 100)
				go func() {
					for i := 0; i < count; i++ {
						resp, err := client.Recv()
						if err != nil {
							return
						}

						c <- resp.Payload
					}
				}()
				return c
			}

			BeforeEach(func(done Done) {
				defer close(done)
				dopplerA = newMockDopplerServer()
				dopplerB = newMockDopplerServer()

				grpcServerURIa := createGrpcServer(dopplerA)
				grpcServerURIb := createGrpcServer(dopplerB)

				mockFinder.GrpcURIsOutput.Ret0 <- []string{grpcServerURIa, grpcServerURIb}
			})

			It("establishes a connection to both servers", func() {
				client, err := connector.Stream(context.Background(), &plumbing.StreamRequest{"AppID"})
				Expect(err).ToNot(HaveOccurred())

				payloads := fetchPayloads(2, client)
				Eventually(payloads).Should(Receive())
				Eventually(payloads).Should(Receive())
			})
		})

		Context("when a new server is advertised", func() {
			It("establishes a connection to the new server", func() {

			})
		})

		Context("when a server goes down and comes back", func() {
			It("establishes a new connection to the server", func() {

			})
		})
	})
})
