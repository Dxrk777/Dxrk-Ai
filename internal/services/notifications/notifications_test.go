package notifications

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNotificationCreation(t *testing.T) {
	n := Notification{
		ID:       "test-1",
		Type:     TypeInfo,
		Title:    "Test",
		Message:  "Hello",
		Priority: PriorityNormal,
		Source:   "test",
	}

	if n.ID != "test-1" {
		t.Errorf("expected ID test-1, got %s", n.ID)
	}
	if n.Type != TypeInfo {
		t.Errorf("expected type TypeInfo, got %d", n.Type)
	}
}

func TestNotificationTypeString(t *testing.T) {
	tests := []struct {
		t    NotificationType
		want string
	}{
		{TypeInfo, "info"},
		{TypeSuccess, "success"},
		{TypeWarning, "warning"},
		{TypeError, "error"},
		{TypeProgress, "progress"},
		{NotificationType(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.t.String(); got != tt.want {
			t.Errorf("NotificationType(%d).String() = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestPriorityString(t *testing.T) {
	tests := []struct {
		p    Priority
		want string
	}{
		{PriorityLow, "low"},
		{PriorityNormal, "normal"},
		{PriorityHigh, "high"},
		{PriorityUrgent, "urgent"},
		{Priority(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.p.String(); got != tt.want {
			t.Errorf("Priority(%d).String() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestNotifierDispatch(t *testing.T) {
	n := NewNotifier(NotifierConfig{})
	ch := NewInMemoryChannel()
	n.RegisterChannel(ch)

	err := n.Info("Test", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notifs := ch.GetNotifications()
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	if notifs[0].Title != "Test" {
		t.Errorf("expected title Test, got %s", notifs[0].Title)
	}
	if notifs[0].Type != TypeInfo {
		t.Errorf("expected type TypeInfo, got %d", notifs[0].Type)
	}
	if notifs[0].ID == "" {
		t.Error("expected auto-generated ID")
	}
	if notifs[0].Timestamp.IsZero() {
		t.Error("expected auto-set timestamp")
	}
}

func TestNotifierChannelDelivery(t *testing.T) {
	n := NewNotifier(NotifierConfig{})
	mem := NewInMemoryChannel()
	n.RegisterChannel(mem)

	n.Success("Deploy", "Complete")
	n.Warning("Disk", "Low space")
	n.Error("API", "Timeout")

	notifs := mem.GetNotifications()
	if len(notifs) != 3 {
		t.Fatalf("expected 3 notifications, got %d", len(notifs))
	}

	if notifs[0].Type != TypeSuccess {
		t.Errorf("expected TypeSuccess, got %d", notifs[0].Type)
	}
	if notifs[1].Type != TypeWarning {
		t.Errorf("expected TypeWarning, got %d", notifs[1].Type)
	}
	if notifs[2].Type != TypeError {
		t.Errorf("expected TypeError, got %d", notifs[2].Type)
	}
}

func TestNotifierTypeFiltering(t *testing.T) {
	n := NewNotifier(NotifierConfig{})
	progressOnly := &filteredChannel{
		name:     "progress_only",
		supports: TypeProgress,
	}
	n.RegisterChannel(progressOnly)

	n.Info("Test", "This should not be delivered")
	n.Progress("Build", 1, 10)

	if progressOnly.count != 1 {
		t.Errorf("expected 1 delivery, got %d", progressOnly.count)
	}
}

func TestNotifierHistory(t *testing.T) {
	n := NewNotifier(NotifierConfig{MaxHistory: 5})
	n.Info("Test", "1")
	n.Info("Test", "2")
	n.Info("Test", "3")

	history := n.History(2)
	if len(history) != 2 {
		t.Fatalf("expected 2 history items, got %d", len(history))
	}
	if history[0].Message != "2" {
		t.Errorf("expected message 2, got %s", history[0].Message)
	}

	allHistory := n.History(0)
	if len(allHistory) != 3 {
		t.Errorf("expected 3 history items, got %d", len(allHistory))
	}
}

func TestNotifierMarkRead(t *testing.T) {
	n := NewNotifier(NotifierConfig{})
	n.Info("Test", "1")
	n.Info("Test", "2")

	if n.UnreadCount() != 2 {
		t.Errorf("expected 2 unread, got %d", n.UnreadCount())
	}

	history := n.History(1)
	found := n.MarkRead(history[0].ID)
	if !found {
		t.Error("expected MarkRead to return true")
	}

	if n.UnreadCount() != 1 {
		t.Errorf("expected 1 unread, got %d", n.UnreadCount())
	}

	found = n.MarkRead("nonexistent")
	if found {
		t.Error("expected MarkRead to return false for nonexistent ID")
	}
}

func TestNotifierClearHistory(t *testing.T) {
	n := NewNotifier(NotifierConfig{})
	n.Info("Test", "1")
	n.Info("Test", "2")

	n.ClearHistory()
	history := n.History(0)
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d", len(history))
	}
}

func TestNotifierHistoryMaxLimit(t *testing.T) {
	n := NewNotifier(NotifierConfig{MaxHistory: 3})
	for i := 0; i < 10; i++ {
		n.Info("Test", "msg")
	}

	history := n.History(0)
	if len(history) != 3 {
		t.Errorf("expected 3 history items (max), got %d", len(history))
	}
}

func TestNotifierDeduplication(t *testing.T) {
	n := NewNotifier(NotifierConfig{DedupWindow: time.Second})

	n1 := Notification{
		Type:    TypeInfo,
		Title:   "Test",
		Source:  "test",
		Message: "Hello",
	}
	n2 := Notification{
		Type:    TypeInfo,
		Title:   "Test",
		Source:  "test",
		Message: "Hello",
	}

	if n.Deduplicate(n1) {
		t.Error("first notification should not be a duplicate")
	}
	if !n.Deduplicate(n2) {
		t.Error("second notification should be a duplicate")
	}
}

func TestNotifierDeduplicationExpiry(t *testing.T) {
	n := NewNotifier(NotifierConfig{DedupWindow: 10 * time.Millisecond})

	n1 := Notification{
		Type:    TypeInfo,
		Title:   "Test",
		Source:  "test",
		Message: "Hello",
	}

	if n.Deduplicate(n1) {
		t.Error("first notification should not be a duplicate")
	}

	time.Sleep(20 * time.Millisecond)

	n2 := Notification{
		Type:    TypeInfo,
		Title:   "Test",
		Source:  "test",
		Message: "Hello",
	}
	if n.Deduplicate(n2) {
		t.Error("notification should not be a duplicate after dedup window")
	}
}

func TestNotifierProgress(t *testing.T) {
	n := NewNotifier(NotifierConfig{})
	ch := NewInMemoryChannel()
	n.RegisterChannel(ch)

	n.Progress("Build", 5, 10)

	notifs := ch.GetNotifications()
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}

	if notifs[0].Type != TypeProgress {
		t.Errorf("expected TypeProgress, got %d", notifs[0].Type)
	}
	if notifs[0].Message != "5/10 (50%)" {
		t.Errorf("expected progress message 5/10 (50%%), got %s", notifs[0].Message)
	}
	if notifs[0].Details["current"] != 5 {
		t.Errorf("expected current=5, got %v", notifs[0].Details["current"])
	}
}

func TestNotifierRetry(t *testing.T) {
	n := NewNotifier(NotifierConfig{MaxRetries: 2, RetryDelay: time.Millisecond})
	ch := &failingChannel{
		name:      "failing",
		failCount: 2,
	}
	n.RegisterChannel(ch)

	err := n.Info("Test", "Hello")
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if ch.attempts != 3 {
		t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", ch.attempts)
	}
}

func TestNotifierRetryExhausted(t *testing.T) {
	n := NewNotifier(NotifierConfig{MaxRetries: 2, RetryDelay: time.Millisecond})
	ch := &failingChannel{
		name:      "failing",
		failCount: 10,
	}
	n.RegisterChannel(ch)

	err := n.Info("Test", "Hello")
	if err == nil {
		t.Error("expected error after retries exhausted")
	}
}

func TestAggregatorAdd(t *testing.T) {
	a := NewAggregator(time.Second)

	n1 := Notification{
		Type:      TypeInfo,
		Title:     "Test",
		Source:    "svc",
		Message:   "Hello",
		Timestamp: time.Now(),
	}

	added := a.Add(n1)
	if !added {
		t.Error("expected notification to be added")
	}

	groups := a.Flush()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Count != 1 {
		t.Errorf("expected count 1, got %d", groups[0].Count)
	}
}

func TestAggregatorSuppression(t *testing.T) {
	a := NewAggregator(100 * time.Millisecond)

	for i := 0; i < 15; i++ {
		n := Notification{
			Type:      TypeInfo,
			Title:     "Test",
			Source:    "svc",
			Message:   "msg",
			Timestamp: time.Now(),
		}
		a.Add(n)
	}

	groups := a.Flush()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	if groups[0].Suppressed == 0 {
		t.Error("expected some suppressed notifications")
	}
	if groups[0].Count <= 10 {
		t.Errorf("expected count > 10, got %d", groups[0].Count)
	}
}

func TestAggregatorFlush(t *testing.T) {
	a := NewAggregator(time.Second)

	a.Add(Notification{Type: TypeInfo, Title: "A", Source: "svc1", Timestamp: time.Now()})
	a.Add(Notification{Type: TypeInfo, Title: "B", Source: "svc2", Timestamp: time.Now()})

	groups := a.Flush()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Second flush should be empty
	groups = a.Flush()
	if len(groups) != 0 {
		t.Errorf("expected empty flush, got %d groups", len(groups))
	}
}

func TestAggregatorSummary(t *testing.T) {
	a := NewAggregator(time.Second)

	a.Add(Notification{Type: TypeInfo, Title: "A", Source: "svc", Timestamp: time.Now()})
	a.Add(Notification{Type: TypeSuccess, Title: "B", Source: "svc", Timestamp: time.Now()})

	key := "svc:0" // TypeInfo = 0
	summary := a.Summary(key)
	if summary == "no notifications" {
		t.Error("expected non-empty summary")
	}
}

func TestAggregatorShouldSuppress(t *testing.T) {
	a := NewAggregator(time.Hour)

	for i := 0; i < 10; i++ {
		a.Add(Notification{Type: TypeInfo, Title: "Test", Source: "svc", Timestamp: time.Now()})
	}

	if !a.ShouldSuppress("svc:0", 10) {
		t.Error("expected ShouldSuppress to return true")
	}

	if a.ShouldSuppress("svc:0", 20) {
		t.Error("expected ShouldSuppress to return false for higher limit")
	}

	if a.ShouldSuppress("nonexistent", 10) {
		t.Error("expected ShouldSuppress to return false for nonexistent group")
	}
}

func TestConsoleChannel(t *testing.T) {
	var buf bytes.Buffer
	ch := NewConsoleChannel(&buf, false)

	n := Notification{
		Type:      TypeInfo,
		Title:     "Test",
		Message:   "Hello",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	err := ch.Send(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected non-empty output")
	}
	if output != "[i] [12:00:00] Test: Hello\n" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestConsoleChannelColorized(t *testing.T) {
	var buf bytes.Buffer
	ch := NewConsoleChannel(&buf, true)

	n := Notification{
		Type:      TypeError,
		Title:     "Fail",
		Message:   "Broken",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	err := ch.Send(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected non-empty output")
	}
	// Check for ANSI color code
	if !strings.Contains(output, "\033[31m") {
		t.Error("expected ANSI color codes in output")
	}
}

func TestCallbackChannel(t *testing.T) {
	var received Notification
	ch := NewCallbackChannel(func(n Notification) {
		received = n
	})

	n := Notification{Title: "Test", Message: "Hello"}
	err := ch.Send(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.Title != "Test" {
		t.Errorf("expected title Test, got %s", received.Title)
	}
}

func TestCallbackChannelNil(t *testing.T) {
	ch := NewCallbackChannel(nil)
	err := ch.Send(Notification{})
	if err == nil {
		t.Error("expected error for nil callback")
	}
}

func TestBufferedChannel(t *testing.T) {
	ch := NewBufferedChannel(2)

	n := Notification{Title: "Test"}
	err := ch.Send(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	received := <-ch.Receive()
	if received.Title != "Test" {
		t.Errorf("expected title Test, got %s", received.Title)
	}
}

func TestBufferedChannelFull(t *testing.T) {
	ch := NewBufferedChannel(1)
	ch.Send(Notification{Title: "1"})

	err := ch.Send(Notification{Title: "2"})
	if err == nil {
		t.Error("expected error when channel is full")
	}
}

func TestBufferedChannelDefaultCapacity(t *testing.T) {
	ch := NewBufferedChannel(0)
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
}

func TestNotifierConcurrentAccess(t *testing.T) {
	n := NewNotifier(NotifierConfig{MaxHistory: 100})
	ch := NewInMemoryChannel()
	n.RegisterChannel(ch)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n.Info("Test", "msg")
		}(i)
	}
	wg.Wait()

	notifs := ch.GetNotifications()
	if len(notifs) != 50 {
		t.Errorf("expected 50 notifications, got %d", len(notifs))
	}
}

func TestAggregatorConcurrentAccess(t *testing.T) {
	a := NewAggregator(time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.Add(Notification{
				Type:      TypeInfo,
				Title:     "Test",
				Source:    "svc",
				Message:   "msg",
				Timestamp: time.Now(),
			})
		}()
	}
	wg.Wait()
}

type filteredChannel struct {
	name     string
	supports NotificationType
	count    int
}

func (c *filteredChannel) Name() string { return c.name }

func (c *filteredChannel) Send(n Notification) error {
	c.count++
	return nil
}

func (c *filteredChannel) SupportsType(t NotificationType) bool {
	return t == c.supports
}

type failingChannel struct {
	name      string
	failCount int
	attempts  int
}

func (c *failingChannel) Name() string { return c.name }

func (c *failingChannel) Send(n Notification) error {
	c.attempts++
	if c.attempts <= c.failCount {
		return fmt.Errorf("simulated failure")
	}
	return nil
}

func (c *failingChannel) SupportsType(_ NotificationType) bool {
	return true
}
