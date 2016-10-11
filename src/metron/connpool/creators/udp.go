package creators

import (
	"io"
	"log"
	"math/rand"
	"net"
)

type UDPFinder interface {
	UDPServers() []string
}

type UDP struct {
	retryCount int
	finder     UDPFinder
}

func NewUDP(retryCount int, finder UDPFinder) *UDP {
	return &UDP{
		retryCount: retryCount,
		finder:     finder,
	}
}

func (t *UDP) Create() io.WriteCloser {
	for i := 0; i < t.retryCount; i++ {
		URIs := t.finder.UDPServers()
		if len(URIs) == 0 {
			log.Print("did not find any UDP connections")
			return nil
		}

		URI := URIs[rand.Intn(len(URIs))]
		addr, err := net.ResolveUDPAddr("udp", URI)
		if err != nil {
			log.Printf("invalid address: %s", addr)
			continue
		}

		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			log.Printf("connection failed to '%s': %s", URI, err)
			continue
		}

		log.Printf("connection succeeded to '%s'", URI)
		return conn
	}

	return nil
}
