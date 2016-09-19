package fake_doppler

import (
	"net"
	"net/http"
	"plumbing"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	gorilla "github.com/gorilla/websocket"
)

type FakeDoppler struct {
	ApiEndpoint                string
	GrpcEndpoint               string
	connectionListener         net.Listener
	grpcListener               net.Listener
	websocket                  *gorilla.Conn
	sendMessageChan            chan []byte
	sendGrpcMessageChan        chan []byte
	TrafficControllerConnected chan *http.Request
	GrpcStreamRequestChan      chan *plumbing.StreamRequest
	GrpcFirehoseRequestChan    chan *plumbing.FirehoseRequest
	connectionPresent          bool
	done                       chan struct{}
	sync.RWMutex
}

func New() *FakeDoppler {
	return &FakeDoppler{
		ApiEndpoint:                "127.0.0.1:1235",
		GrpcEndpoint:               "127.0.0.1:1236",
		TrafficControllerConnected: make(chan *http.Request, 1),
		sendMessageChan:            make(chan []byte, 100),
		sendGrpcMessageChan:        make(chan []byte, 100),
		GrpcStreamRequestChan:      make(chan *plumbing.StreamRequest, 100),
		GrpcFirehoseRequestChan:    make(chan *plumbing.FirehoseRequest, 100),
		done: make(chan struct{}),
	}
}

func (fakeDoppler *FakeDoppler) Start() error {
	var err error
	fakeDoppler.Lock()
	fakeDoppler.grpcListener, err = net.Listen("tcp", fakeDoppler.GrpcEndpoint)
	fakeDoppler.Unlock()
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	plumbing.RegisterDopplerServer(grpcServer, fakeDoppler)
	go grpcServer.Serve(fakeDoppler.grpcListener)

	fakeDoppler.Lock()
	fakeDoppler.connectionListener, err = net.Listen("tcp", fakeDoppler.ApiEndpoint)
	fakeDoppler.Unlock()
	if err != nil {
		return err
	}
	s := &http.Server{Addr: fakeDoppler.ApiEndpoint, Handler: fakeDoppler}

	err = s.Serve(fakeDoppler.connectionListener)
	close(fakeDoppler.done)
	return err
}

func (fakeDoppler *FakeDoppler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	select {
	case fakeDoppler.TrafficControllerConnected <- request:
	default:
	}

	fakeDoppler.Lock()
	fakeDoppler.connectionPresent = true
	fakeDoppler.Unlock()

	handlers.NewWebsocketHandler(fakeDoppler.sendMessageChan, time.Millisecond*100, loggertesthelper.Logger()).ServeHTTP(writer, request)

	fakeDoppler.Lock()
	fakeDoppler.connectionPresent = false
	fakeDoppler.Unlock()
}

func (fakeDoppler *FakeDoppler) Stop() {
	fakeDoppler.Lock()
	if fakeDoppler.connectionListener != nil {
		fakeDoppler.connectionListener.Close()
	}
	if fakeDoppler.grpcListener != nil {
		fakeDoppler.grpcListener.Close()
	}
	fakeDoppler.Unlock()
	<-fakeDoppler.done
}

func (fakeDoppler *FakeDoppler) ConnectionPresent() bool {
	fakeDoppler.Lock()
	defer fakeDoppler.Unlock()

	return fakeDoppler.connectionPresent
}

func (fakeDoppler *FakeDoppler) SendLogMessage(messageBody []byte) {
	fakeDoppler.sendMessageChan <- messageBody
}

// TODO: Remove ViaGrpc suffix when WS endpoints on doppler are removed
func (fakeDoppler *FakeDoppler) SendLogMessageViaGrpc(messageBody []byte) {
	fakeDoppler.sendGrpcMessageChan <- messageBody
}

func (fakeDoppler *FakeDoppler) CloseLogMessageStream() {
	close(fakeDoppler.sendMessageChan)
}

func (fakeDoppler *FakeDoppler) Stream(request *plumbing.StreamRequest, server plumbing.Doppler_StreamServer) error {
	fakeDoppler.GrpcStreamRequestChan <- request
	for msg := range fakeDoppler.sendGrpcMessageChan {
		err := server.Send(&plumbing.Response{
			Payload: msg,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (fakeDoppler *FakeDoppler) Firehose(request *plumbing.FirehoseRequest, server plumbing.Doppler_FirehoseServer) error {
	fakeDoppler.GrpcFirehoseRequestChan <- request
	for msg := range fakeDoppler.sendGrpcMessageChan {
		err := server.Send(&plumbing.Response{
			Payload: msg,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
