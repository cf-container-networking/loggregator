package creators

import (
	"io"
	"log"
	"math/rand"
	"net"
)

type TCPFinder interface {
	TCPServers() []string
}

type TCP struct {
	retryCount int
	finder     TCPFinder
}

func NewTCP(retryCount int, finder TCPFinder) *TCP {
	return &TCP{
		retryCount: retryCount,
		finder:     finder,
	}
}

func (t *TCP) Create() io.WriteCloser {
	for i := 0; i < t.retryCount; i++ {
		URIs := t.finder.TCPServers()
		if len(URIs) == 0 {
			log.Print("did not find any TCP connections")
			return nil
		}

		URI := URIs[rand.Intn(len(URIs))]
		conn, err := net.Dial("tcp", URI)
		if err != nil {
			log.Printf("connection failed to '%s': %s", URI, err)
			continue
		}

		log.Printf("connection succeeded to '%s'", URI)
		return conn
	}

	return nil
}
