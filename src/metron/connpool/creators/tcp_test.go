package creators_test

import (
	"metron/connpool/creators"
	"net"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TCP", func() {
	var (
		mockFinder *mockTCPFinder

		conns    chan net.Conn
		connURIs chan string

		listeners []net.Listener
		URIs      []string

		tcp *creators.TCP
	)

	var handleListener = func(
		lis net.Listener,
		conns chan<- net.Conn,
		connURIs chan<- string,
	) {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			conns <- conn
			connURIs <- lis.Addr().String()
		}
	}

	var startServer = func(conns chan<- net.Conn, connURIs chan<- string) net.Listener {
		lis, err := net.Listen("tcp", ":0")
		Expect(err).ToNot(HaveOccurred())
		go handleListener(lis, conns, connURIs)
		return lis
	}

	BeforeEach(func() {
		mockFinder = newMockTCPFinder()
		conns = make(chan net.Conn, 100)
		connURIs = make(chan string, 100)

		tcp = creators.NewTCP(10, mockFinder)

		for i := 0; i < 3; i++ {
			lis := startServer(conns, connURIs)
			listeners = append(listeners, lis)
			URIs = append(URIs, lis.Addr().String())
		}
	})

	AfterEach(func() {
		for _, lis := range listeners {
			lis.Close()
		}
		listeners = nil
		URIs = nil
	})

	Describe("Create()", func() {
		Context("Finder returns TCP URIs", func() {
			var mapURIs = func(count int, connURIs <-chan string) map[string]int {
				m := make(map[string]int)
				for i := 0; i < count; i++ {
					URI := <-connURIs
					m[URI]++
				}
				return m
			}

			BeforeEach(func() {
				testhelpers.AlwaysReturn(mockFinder.TCPServersOutput.Ret0, URIs)
			})

			It("connects to each TCP server semi-equally", func(done Done) {
				defer close(done)
				for i := 0; i < 100; i++ {
					tcp.Create()
				}

				m := mapURIs(100, connURIs)
				Expect(m).To(HaveLen(len(URIs)))
				for _, URI := range URIs {
					Expect(m[URI]).To(BeNumerically("~", 100/len(URIs), 10))
				}
			})

			Context("Connection causes an error", func() {
				BeforeEach(func() {
					listeners[0].Close()
				})

				It("does not return nil", func() {
					Consistently(tcp.Create).ShouldNot(BeNil())
				})

				Context("all connections return an error", func() {
					BeforeEach(func() {
						listeners[1].Close()
						listeners[2].Close()
					})

					It("tries a few times then returns a nil", func() {
						Eventually(tcp.Create, 5).Should(BeNil())
					})
				})
			})
		})

		Context("Finder does not return any URIs", func() {
			BeforeEach(func() {
				close(mockFinder.TCPServersOutput.Ret0)
			})

			It("returns nil", func(done Done) {
				defer close(done)
				Expect(tcp.Create()).To(BeNil())
			})
		})
	})
})
