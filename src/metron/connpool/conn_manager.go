package connpool

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
	"unsafe"
)

var ConnNotAvailableErr = fmt.Errorf("Connection not available")

type ConnManager struct {
	killAfter int64
	count     int64
	creator   ConnCreator
	writer    unsafe.Pointer
}

func NewConnManager(killAfter int, creator ConnCreator) *ConnManager {
	m := &ConnManager{
		killAfter: int64(killAfter),
		creator:   creator,
	}

	go m.run()
	return m
}

func (m *ConnManager) Write(data []byte) (int, error) {
	writer := atomic.LoadPointer(&m.writer)
	if writer == nil {
		return 0, ConnNotAvailableErr
	}

	w := *(*io.WriteCloser)(writer)
	if w == nil {
		return 0, ConnNotAvailableErr
	}

	n, err := w.Write(data)
	if err != nil {
		m.reset()
		return 0, err
	}

	if atomic.AddInt64(&m.count, 1) >= m.killAfter {
		m.reset()
	}

	return n, nil
}

func (m *ConnManager) reset() {
	atomic.StorePointer(&m.writer, nil)
	atomic.StoreInt64(&m.count, 0)
}

func (m *ConnManager) run() {
	for range time.Tick(50 * time.Millisecond) {
		writer := atomic.LoadPointer(&m.writer)
		if writer != nil && *(*io.WriteCloser)(writer) != nil {
			continue
		}

		newWriter := m.creator.Create()
		atomic.StorePointer(&m.writer, unsafe.Pointer(&newWriter))
	}
}
