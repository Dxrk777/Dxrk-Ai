package advanced

import (
	"sync"
)

type EventType string

const (
	EventSessionStarted   EventType = "session.started"
	EventSessionEnded     EventType = "session.ended"
	EventSessionFailed    EventType = "session.failed"
	EventActivity         EventType = "activity"
	EventPermissionReq    EventType = "permission.request"
	EventPermissionResp   EventType = "permission.response"
	EventTransportConnect EventType = "transport.connect"
	EventTransportClose   EventType = "transport.close"
	EventHealthChange     EventType = "health.change"
	EventCustom           EventType = "custom"
)

type Event struct {
	Type      EventType
	Source    string
	SessionID string
	Payload   interface{}
}

type EventHandler func(Event)

type subscription struct {
	id      uint64
	event   EventType
	handler EventHandler
	once    bool
}

type EventBus struct {
	mu           sync.RWMutex
	subs         map[uint64]*subscription
	nextID       uint64
	globalSubs   []EventHandler
	sessionSubs  map[string][]*subscription
	sourceFilter map[string]bool
	bufferSize   int
}

type EventBusOption func(*EventBus)

func WithEventBufferSize(size int) EventBusOption {
	return func(b *EventBus) { b.bufferSize = size }
}

func WithSourceFilter(sources ...string) EventBusOption {
	return func(b *EventBus) {
		b.sourceFilter = make(map[string]bool, len(sources))
		for _, s := range sources {
			b.sourceFilter[s] = true
		}
	}
}

func NewEventBus(opts ...EventBusOption) *EventBus {
	b := &EventBus{
		subs:         make(map[uint64]*subscription),
		sessionSubs:  make(map[string][]*subscription),
		sourceFilter: make(map[string]bool),
		bufferSize:   256,
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

func (b *EventBus) Subscribe(event EventType, handler EventHandler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	b.subs[id] = &subscription{id: id, event: event, handler: handler}
	return func() { b.Unsubscribe(id) }
}

func (b *EventBus) SubscribeOnce(event EventType, handler EventHandler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	b.subs[id] = &subscription{id: id, event: event, handler: handler, once: true}
	return func() { b.Unsubscribe(id) }
}

func (b *EventBus) SubscribeGlobal(handler EventHandler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	b.subs[id] = &subscription{id: id, event: "", handler: handler}
	b.globalSubs = append(b.globalSubs, handler)
	return func() { b.UnsubscribeGlobal(id) }
}

func (b *EventBus) SubscribeSession(sessionID string, handler EventHandler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	sub := &subscription{id: id, event: "", handler: handler}
	b.sessionSubs[sessionID] = append(b.sessionSubs[sessionID], sub)
	return func() { b.UnsubscribeSession(sessionID, id) }
}

func (b *EventBus) Unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs, id)
}

func (b *EventBus) UnsubscribeGlobal(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs, id)
	for i, h := range b.globalSubs {
		_ = h
		if i < len(b.globalSubs) {
			b.globalSubs = append(b.globalSubs[:i], b.globalSubs[i+1:]...)
			break
		}
	}
}

func (b *EventBus) UnsubscribeSession(sessionID string, id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs, id)
	subs := b.sessionSubs[sessionID]
	for i, s := range subs {
		if s.id == id {
			b.sessionSubs[sessionID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(b.sessionSubs[sessionID]) == 0 {
		delete(b.sessionSubs, sessionID)
	}
}

func (b *EventBus) Publish(evt Event) {
	if len(b.sourceFilter) > 0 && evt.Source != "" {
		if !b.sourceFilter[evt.Source] {
			return
		}
	}

	b.mu.RLock()
	var matched []*subscription
	for _, sub := range b.subs {
		if sub.event == "" || sub.event == evt.Type {
			matched = append(matched, sub)
		}
	}
	globalHandlers := make([]EventHandler, len(b.globalSubs))
	copy(globalHandlers, b.globalSubs)

	var sessionHandlers []EventHandler
	if evt.SessionID != "" {
		subs := b.sessionSubs[evt.SessionID]
		for _, s := range subs {
			sessionHandlers = append(sessionHandlers, s.handler)
		}
	}
	b.mu.RUnlock()

	for _, sub := range matched {
		sub.handler(evt)
		if sub.once {
			b.Unsubscribe(sub.id)
		}
	}
	for _, h := range globalHandlers {
		h(evt)
	}
	for _, h := range sessionHandlers {
		h(evt)
	}
}

func (b *EventBus) PublishAsync(evt Event) {
	go b.Publish(evt)
}

func (b *EventBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

func (b *EventBus) SessionSubscriberCount(sessionID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.sessionSubs[sessionID])
}

func (b *EventBus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = make(map[uint64]*subscription)
	b.globalSubs = nil
	b.sessionSubs = make(map[string][]*subscription)
}

func (b *EventBus) Emit(sessionID string, evtType EventType, payload interface{}) {
	b.Publish(Event{
		Type:      evtType,
		SessionID: sessionID,
		Payload:   payload,
	})
}
