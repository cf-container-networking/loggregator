package grpcmanager

import (
	"math/rand"
	"plumbing"
	"sync"

	"github.com/cloudfoundry/sonde-go/events"
)

type Router struct {
	lock sync.RWMutex

	// First Key is AppID, Second Key is ShardID
	// Empty AppID means no filter
	subscriptions map[string]map[string][]DataSetter
}

func NewRouter() *Router {
	return &Router{
		subscriptions: make(map[string]map[string][]DataSetter),
	}
}

func (r *Router) Register(req *plumbing.SubscriptionRequest, dataSetter DataSetter) (cleanup func()) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.registerSetter(req, dataSetter)

	return r.buildCleanup(req, dataSetter)
}

func (r *Router) SendTo(appID string, envelope *events.Envelope) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	data := r.marshal(envelope)

	if data == nil {
		return
	}

	for shardID, setters := range r.subscriptions[appID] {
		r.writeToShard(shardID, setters, data)
	}

	for shardID, setters := range r.subscriptions[""] {
		r.writeToShard(shardID, setters, data)
	}
}

func (r *Router) writeToShard(shardID string, setters []DataSetter, data []byte) {
	if shardID == "" {
		for _, setter := range setters {
			setter.Set(data)
		}
		return
	}

	setters[rand.Intn(len(setters))].Set(data)
}

func (r *Router) registerSetter(req *plumbing.SubscriptionRequest, dataSetter DataSetter) {
	var appID string
	if req.Filter != nil {
		appID = req.Filter.AppID
	}

	m, ok := r.subscriptions[appID]
	if !ok {
		m = make(map[string][]DataSetter)
		r.subscriptions[appID] = m
	}

	m[req.ShardID] = append(m[req.ShardID], dataSetter)
}

func (r *Router) buildCleanup(req *plumbing.SubscriptionRequest, dataSetter DataSetter) func() {
	return func() {
		r.lock.Lock()
		defer r.lock.Unlock()

		var appID string
		if req.Filter != nil {
			appID = req.Filter.AppID
		}

		var setters []DataSetter
		for _, s := range r.subscriptions[appID][req.ShardID] {
			if s != dataSetter {
				setters = append(setters, s)
			}
		}

		if len(setters) > 0 {
			r.subscriptions[appID][req.ShardID] = setters
			return
		}

		delete(r.subscriptions[appID], req.ShardID)

		if len(r.subscriptions[appID]) == 0 {
			delete(r.subscriptions, appID)
		}
	}
}

func (r *Router) marshal(envelope *events.Envelope) []byte {
	data, err := envelope.Marshal()
	if err != nil {
		return nil
	}

	return data
}
