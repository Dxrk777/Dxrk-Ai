package advanced

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

var (
	ErrTransportClosed  = errors.New("transport closed")
	ErrCodecUnsupported = errors.New("codec not supported")
	ErrMessageTooLarge  = errors.New("message too large")
)

type Codec interface {
	Marshal(v interface{}) ([]byte, error)
	MarshalTo(w io.Writer, v interface{}) (int, error)
	Unmarshal(data []byte, v interface{}) error
	UnmarshalFrom(r io.Reader, v interface{}) error
	Name() string
}

type JSONCodec struct{}

func (c JSONCodec) Marshal(v interface{}) ([]byte, error) { return json.Marshal(v) }
func (c JSONCodec) MarshalTo(w io.Writer, v interface{}) (int, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	return w.Write(data)
}
func (c JSONCodec) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }
func (c JSONCodec) UnmarshalFrom(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}
func (c JSONCodec) Name() string { return "json" }

type TransportState int

const (
	TransportDisconnected TransportState = iota
	TransportConnecting
	TransportConnected
	TransportClosing
	TransportClosed
)

func (s TransportState) String() string {
	switch s {
	case TransportDisconnected:
		return "disconnected"
	case TransportConnecting:
		return "connecting"
	case TransportConnected:
		return "connected"
	case TransportClosing:
		return "closing"
	case TransportClosed:
		return "closed"
	default:
		return strconst.StrUnknown
	}
}

type TransportMessage struct {
	ID        string
	Type      string
	Payload   []byte
	Timestamp time.Time
	Metadata  map[string]string
}

type Transport interface {
	Send(ctx context.Context, msg *TransportMessage) error
	SendBatch(ctx context.Context, msgs []*TransportMessage) error
	Receive(ctx context.Context) (*TransportMessage, error)
	Close() error
	State() TransportState
	RemoteAddr() net.Addr
	SetStateHandler(fn func(TransportState))
	SetErrorHandler(fn func(error))
}

type TCPTransport struct {
	mu           sync.RWMutex
	conn         net.Conn
	codec        Codec
	state        TransportState
	stateHandler func(TransportState)
	errorHandler func(error)
	readBuf      []byte
	writeBuf     []byte
	maxMsgSize   int
	quit         chan struct{}
	done         chan struct{}
}

type TCPTransportOption func(*TCPTransport)

func WithTCPMaxMessageSize(size int) TCPTransportOption {
	return func(t *TCPTransport) { t.maxMsgSize = size }
}

func NewTCPTransport(conn net.Conn, codec Codec, opts ...TCPTransportOption) *TCPTransport {
	t := &TCPTransport{
		conn:       conn,
		codec:      codec,
		state:      TransportConnected,
		readBuf:    make([]byte, 64*1024),
		writeBuf:   make([]byte, 64*1024),
		maxMsgSize: 10 * 1024 * 1024,
		quit:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

func (t *TCPTransport) Send(ctx context.Context, msg *TransportMessage) error {
	t.mu.RLock()
	if t.state != TransportConnected {
		t.mu.RUnlock()
		return ErrTransportClosed
	}
	t.mu.RUnlock()

	data, err := t.codec.Marshal(msg)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != TransportConnected {
		return ErrTransportClosed
	}

	if len(data) > t.maxMsgSize {
		return ErrMessageTooLarge
	}

	sizeBuf := []byte{
		byte(len(data) >> 24),
		byte(len(data) >> 16),
		byte(len(data) >> 8),
		byte(len(data)),
	}

	if _, err := t.conn.Write(sizeBuf); err != nil {
		return err
	}
	if _, err := t.conn.Write(data); err != nil {
		return err
	}

	return nil
}

func (t *TCPTransport) SendBatch(ctx context.Context, msgs []*TransportMessage) error {
	for _, msg := range msgs {
		if err := t.Send(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func (t *TCPTransport) Receive(ctx context.Context) (*TransportMessage, error) {
	t.mu.RLock()
	if t.state != TransportConnected {
		t.mu.RUnlock()
		return nil, ErrTransportClosed
	}
	conn := t.conn
	t.mu.RUnlock()

	sizeBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, sizeBuf); err != nil {
		return nil, err
	}

	size := int(sizeBuf[0])<<24 | int(sizeBuf[1])<<16 | int(sizeBuf[2])<<8 | int(sizeBuf[3])
	if size > t.maxMsgSize {
		return nil, ErrMessageTooLarge
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}

	var msg TransportMessage
	if err := t.codec.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (t *TCPTransport) Close() error {
	t.mu.Lock()
	if t.state == TransportClosed || t.state == TransportClosing {
		t.mu.Unlock()
		return nil
	}
	t.state = TransportClosing
	t.mu.Unlock()

	close(t.quit)
	err := t.conn.Close()

	t.mu.Lock()
	t.state = TransportClosed
	t.mu.Unlock()

	close(t.done)
	return err
}

func (t *TCPTransport) State() TransportState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

func (t *TCPTransport) RemoteAddr() net.Addr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.conn != nil {
		return t.conn.RemoteAddr()
	}
	return nil
}

func (t *TCPTransport) SetStateHandler(fn func(TransportState)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stateHandler = fn
}

func (t *TCPTransport) SetErrorHandler(fn func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errorHandler = fn
}

type HTTPTransport struct {
	mu         sync.RWMutex
	config     *Config
	codec      Codec
	baseURL    string
	token      string
	state      TransportState
	httpClient *http.Client
	quit       chan struct{}
}

type HTTPOption func(*HTTPTransport)

func WithHTTPToken(token string) HTTPOption {
	return func(t *HTTPTransport) { t.token = token }
}

func NewHTTPTransport(config *Config, codec Codec, opts ...HTTPOption) *HTTPTransport {
	t := &HTTPTransport{
		config:  config,
		codec:   codec,
		baseURL: fmt.Sprintf("http://%s", config.Address()),
		state:   TransportConnected,
		quit:    make(chan struct{}),
		httpClient: &http.Client{
			Timeout: config.ConnectTimeout,
			Transport: &http.Transport{
				MaxIdleConns:    config.MaxIdleConns,
				IdleConnTimeout: config.IdleTimeout,
			},
		},
	}
	if config.TLSEnabled {
		t.baseURL = fmt.Sprintf("https://%s", config.Address())
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

func (t *HTTPTransport) Send(ctx context.Context, msg *TransportMessage) error {
	t.mu.RLock()
	if t.state != TransportConnected {
		t.mu.RUnlock()
		return ErrTransportClosed
	}
	t.mu.RUnlock()

	data, err := t.codec.Marshal(msg)
	if err != nil {
		return err
	}

	u := t.baseURL + "/bridge/message"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &byteReader{data: data})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func (t *HTTPTransport) SendBatch(ctx context.Context, msgs []*TransportMessage) error {
	for _, msg := range msgs {
		if err := t.Send(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func (t *HTTPTransport) Receive(ctx context.Context) (*TransportMessage, error) {
	t.mu.RLock()
	if t.state != TransportConnected {
		t.mu.RUnlock()
		return nil, ErrTransportClosed
	}
	t.mu.RUnlock()

	u := t.baseURL + "/bridge/poll"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var msg TransportMessage
	if err := t.codec.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = TransportClosed
	close(t.quit)
	return nil
}

func (t *HTTPTransport) State() TransportState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

func (t *HTTPTransport) RemoteAddr() net.Addr {
	u, err := url.Parse(t.baseURL)
	if err != nil {
		return nil
	}
	host := u.Hostname()
	portStr := u.Port()
	if portStr == "" {
		if u.Scheme == "https" {
			portStr = "443"
		} else {
			portStr = "80"
		}
	}
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)
	return &net.TCPAddr{IP: net.ParseIP(host), Port: port}
}

func (t *HTTPTransport) SetStateHandler(fn func(TransportState)) {}
func (t *HTTPTransport) SetErrorHandler(fn func(error))          {}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
