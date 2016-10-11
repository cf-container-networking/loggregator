package connpool

import (
	"io"
	"time"
)

type ConnCreator interface {
	Create() io.WriteCloser
}

type ConnPool struct {
	creator   ConnCreator
	managers  []*ConnManager
	killAfter int
}

func New(size, killAfter int, creator ConnCreator) *ConnPool {
	p := &ConnPool{
		creator:   creator,
		killAfter: killAfter,
	}

	p.initManagers(size, killAfter, creator)
	return p
}

func (p *ConnPool) Write(data []byte) (int, error) {
	for i := 0; i < p.killAfter; i++ {
		n, err := p.tryWrite(data)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		return n, nil
	}

	return 0, ConnNotAvailableErr
}

func (p *ConnPool) tryWrite(data []byte) (int, error) {
	for i := 0; i < len(p.managers); i++ {
		writer := p.managers[i]
		if writer == nil {
			continue
		}

		n, err := writer.Write(data)
		if err != nil {
			continue
		}

		return n, err
	}

	return 0, ConnNotAvailableErr
}

func (p *ConnPool) initManagers(size, killAfter int, creator ConnCreator) {
	for i := 0; i < size; i++ {
		p.managers = append(p.managers, NewConnManager(killAfter, creator))
	}
}
