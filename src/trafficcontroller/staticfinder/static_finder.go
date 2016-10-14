package staticfinder

import (
	"doppler/dopplerservice"
	"fmt"
)

// THIS IS SPIKE CODE AND NOT TESTED

type StaticFinder struct {
	once chan bool
	URIs []string
}

func New(dopplerURIs []string) *StaticFinder {
	once := make(chan bool, 1)
	once <- true
	return &StaticFinder{
		once: once,
		URIs: makeUdpURIs(dopplerURIs),
	}
}

func (f *StaticFinder) Next() dopplerservice.Event {
	// We only want this to return once, and then block for-ev-er
	<-f.once
	return dopplerservice.Event{
		UDPDopplers: f.URIs,
	}
}

func makeUdpURIs(URIs []string) []string {
	var result []string
	for _, URI := range URIs {
		result = append(result, fmt.Sprintf("udp://%s:1234", URI))
	}
	return result
}
