package registry

import (
	"github.com/pkg/errors"
	"io"
	"log"
	"sync"
)

var (
	registry *Registry
	once     = new(sync.Once)
)

type CallbackMessage interface {
	io.Reader
	GetID() string
}

type registryRequest struct {
	id           string
	callbackChan chan io.Reader
}

type Registry struct {
	channels map[string]chan io.Reader
	lock     *sync.RWMutex
}

func Get() *Registry {
	if registry == nil {
		once.Do(func() {
			registry = &Registry{
				channels: make(map[string]chan io.Reader),
				lock:     new(sync.RWMutex),
			}
		})
	}
	return registry
}

func (r *Registry) CreateCallback(id string) (<-chan io.Reader, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if _, ok := r.channels[id]; ok {
		return nil, errors.Errorf("callback with id %s already exists")
	}
	r.channels[id] = make(chan io.Reader)
	return r.channels[id], nil
}

func (r *Registry) HandleCallback(msg CallbackMessage) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	ch, ok := r.channels[msg.GetID()]
	if !ok {
		return errors.Errorf("No registered callback with id %s", msg.GetID())
	}
	defer delete(r.channels, msg.GetID())
	defer close(ch)
	log.Println("sending message on channel")
	ch <- msg
	return nil
}
