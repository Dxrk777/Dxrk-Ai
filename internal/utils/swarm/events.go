package swarm

import (
	"context"
	"sync"
	"sync/atomic"
)

type EventBus struct {
	handlers map[SwarmEventType][]EventHandler
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	eventCh  chan SwarmEvent
	closed   atomic.Bool
}

func NewEventBus(ctx context.Context) *EventBus {
	ctx, cancel := context.WithCancel(ctx)
	return &EventBus{
		handlers: make(map[SwarmEventType][]EventHandler),
		ctx:      ctx,
		cancel:   cancel,
		eventCh:  make(chan SwarmEvent, 1024),
	}
}

func (eb *EventBus) Subscribe(eventType SwarmEventType, handler EventHandler) func() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
	index := len(eb.handlers[eventType]) - 1
	return func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		if index < len(eb.handlers[eventType]) {
			eb.handlers[eventType] = append(eb.handlers[eventType][:index], eb.handlers[eventType][index+1:]...)
		}
	}
}

func (eb *EventBus) SubscribeAll(handler EventHandler) func() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	for eventType := range eb.handlers {
		eb.handlers[eventType] = append(eb.handlers[eventType], handler)
	}
	return func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		for eventType := range eb.handlers {
			handlers := eb.handlers[eventType]
			for i, h := range handlers {
				if &h == &handler {
					eb.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
					break
				}
			}
		}
	}
}

func (eb *EventBus) Publish(event SwarmEvent) {
	if eb.closed.Load() {
		return
	}
	select {
	case eb.eventCh <- event:
	case <-eb.ctx.Done():
	default:
	}
}

func (eb *EventBus) Start() {
	eb.wg.Add(1)
	go eb.processEvents()
}

func (eb *EventBus) processEvents() {
	defer eb.wg.Done()
	for {
		select {
		case event := <-eb.eventCh:
			eb.dispatch(event)
		case <-eb.ctx.Done():
			return
		}
	}
}

func (eb *EventBus) dispatch(event SwarmEvent) {
	eb.mu.RLock()
	handlers := make([]EventHandler, len(eb.handlers[event.Type]))
	copy(handlers, eb.handlers[event.Type])
	allHandlers := make([]EventHandler, 0)
	for _, hs := range eb.handlers {
		allHandlers = append(allHandlers, hs...)
	}
	eb.mu.RUnlock()

	for _, h := range handlers {
		h(event)
	}
	for _, h := range allHandlers {
		h(event)
	}
}

func (eb *EventBus) Stop() {
	if eb.closed.CompareAndSwap(false, true) {
		eb.cancel()
		eb.wg.Wait()
		close(eb.eventCh)
	}
}

func (eb *EventBus) Len() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	total := 0
	for _, hs := range eb.handlers {
		total += len(hs)
	}
	return total
}
