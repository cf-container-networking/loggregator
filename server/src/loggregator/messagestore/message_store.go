package messagestore

import (
	"bytes"
	"container/ring"
	"encoding/binary"
	"github.com/cloudfoundry/loggregatorlib/logtarget"
	"sync"
)

type Message struct {
	length  uint32
	payload []byte
}

func newNode(size int) *node {
	return &node{entries: ring.New(size)}
}

type node struct {
	entries *ring.Ring
}

func (n *node) addData(d []byte) {
	n.entries.Value = Message{uint32(len(d)), d}
	n.entries = n.entries.Next()
}

func NewMessageStore(size int) *MessageStore {
	return &MessageStore{
		size: size,
		apps: make(map[string]*node),
	}
}

type MessageStore struct {
	size int
	apps map[string]*node
	sync.RWMutex
}

func (ms *MessageStore) Add(data []byte, lt *logtarget.LogTarget) {
	ms.Lock()
	defer ms.Unlock()

	if lt.AppId != "" {
		app, found := ms.apps[lt.AppId]
		if !found {
			app = newNode(ms.size)
			ms.apps[lt.AppId] = app
		}
		app.addData(data)
	}
}

func (ms *MessageStore) DumpFor(lt *logtarget.LogTarget) []byte {
	ms.RLock()
	defer ms.RUnlock()

	buffer := bytes.NewBufferString("")

	writeEntries := func(m interface{}) {
		message, _ := m.(Message)
		if message.length > 0 {
			binary.Write(buffer, binary.BigEndian, message.length)
			buffer.Write(message.payload)
		}
	}
	app, appFound := ms.apps[lt.AppId]
	if !appFound {
		return buffer.Bytes()
	}
	app.entries.Do(writeEntries)
	return buffer.Bytes()
}
