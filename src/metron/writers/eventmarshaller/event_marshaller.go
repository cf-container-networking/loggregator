package eventmarshaller

import (
	"log"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/pebbe/zmq4"
)

//go:generate hel --type BatchChainByteWriter --output mock_writer_test.go

// BatchChainByteWriter is a byte writer than can accept a series
// of metricbatcher.BatchCounterChainer values.  It should add any
// additional tags it needs and send the chainer when the message
// is successfully sent.
type BatchChainByteWriter interface {
	Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) (sentLength int, err error)
}

//go:generate hel --type EventBatcher --output mock_event_batcher_test.go

type EventBatcher interface {
	BatchCounter(name string) (chainer metricbatcher.BatchCounterChainer)
	BatchIncrementCounter(name string)
}

type EventMarshaller struct {
	batcher    EventBatcher
	byteWriter *zmq4.Socket
	logger     *gosteno.Logger
}

func New(batcher EventBatcher, byteWriter *zmq4.Socket, logger *gosteno.Logger) *EventMarshaller {
	return &EventMarshaller{
		batcher:    batcher,
		byteWriter: byteWriter,
		logger:     logger,
	}
}

func (m *EventMarshaller) Write(envelope *events.Envelope) {
	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		m.logger.Errorf("marshalling error: %v", err)
		m.batcher.BatchIncrementCounter("dropsondeMarshaller.marshalErrors")
		return
	}

	_, err = m.byteWriter.SendBytes(envelopeBytes, zmq4.DONTWAIT)
	if err != nil {
		log.Print(err.Error())
	}
}
