package creators_test

import (
	"metron/connpool/creators"
	"net"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UDP", func() {
	var (
		mockFinder *mockUDPFinder

		connURIs chan string

		conns []*net.UDPConn
		URIs  []string

		udp *creators.UDP
	)

	var handleConn = func(
		conn *net.UDPConn,
		connURIs chan<- string,
	) {
		for {
			buf := make([]byte, 1024)
			_, err := conn.Read(buf)
			if err != nil {
				return
			}
			connURIs <- conn.LocalAddr().String()
		}
	}

	var startServer = func(connURIs chan<- string) *net.UDPConn {
		addr, _ := net.ResolveUDPAddr("udp", ":0")
		conn, err := net.ListenUDP("udp", addr)
		Expect(err).ToNot(HaveOccurred())
		go handleConn(conn, connURIs)
		return conn

		// lis, err := net.Listen("udp", ":0")
		// Expect(err).ToNot(HaveOccurred())
		// go handleListener(lis, conns, connURIs, nil)
		// return lis
	}

	BeforeEach(func() {
		mockFinder = newMockUDPFinder()
		connURIs = make(chan string, 100)

		udp = creators.NewUDP(10, mockFinder)

		for i := 0; i < 3; i++ {
			conn := startServer(connURIs)
			conns = append(conns, conn)
			URIs = append(URIs, conn.LocalAddr().String())
		}
	})

	AfterEach(func() {
		for _, conn := range conns {
			conn.Close()
		}
		conns = nil
		URIs = nil
	})

	Describe("Create()", func() {
		Context("Finder returns UDP URIs", func() {
			var mapURIs = func(count int, connURIs <-chan string) map[string]int {
				m := make(map[string]int)
				for i := 0; i < count; i++ {
					URI := <-connURIs
					m[URI]++
				}
				return m
			}

			BeforeEach(func() {
				testhelpers.AlwaysReturn(mockFinder.UDPServersOutput.Ret0, URIs)
			})

			It("connects to each UDP server semi-equally", func(done Done) {
				defer close(done)
				for i := 0; i < 100; i++ {
					writer := udp.Create()
					writer.Write([]byte("some-data"))
				}

				m := mapURIs(100, connURIs)
				Expect(m).To(HaveLen(len(URIs)))
				for _, URI := range URIs {
					Expect(m[URI]).To(BeNumerically("~", 100/len(URIs), 10))
				}
			})

			Context("Connection causes an error", func() {
				BeforeEach(func() {
					mockFinder.UDPServersOutput.Ret0 <- []string{"invalid"}
				})

				It("does not return nil", func() {
					Consistently(udp.Create).ShouldNot(BeNil())
				})
			})
		})

		Context("Finder does not return any URIs", func() {
			BeforeEach(func() {
				close(mockFinder.UDPServersOutput.Ret0)
			})

			It("returns nil", func(done Done) {
				defer close(done)
				Expect(udp.Create()).To(BeNil())
			})
		})
	})
})
