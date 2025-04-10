package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
)

const (
	Version                = "2.0"
	serviceMethodSeparator = "_"

	defaultWriteTimeout = 10 * time.Minute // used if context has no deadline
)

var null = json.RawMessage("null")

// A value of this type can a JSON-RPC request, successful response or
// error response. Which one it is depends on the fields.
type Message struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func (msg *Message) isCall() bool {
	return msg.hasValidID() && msg.Method != ""
}

func (msg *Message) hasValidID() bool {
	return len(msg.ID) > 0 && msg.ID[0] != '{' && msg.ID[0] != '['
}

func (msg *Message) String() string {
	b, err := json.Marshal(msg)
	check.PanicIfErr(err)
	return string(b)
}

func (msg *Message) errorResponse(err error) *Message {
	resp := errorMessage(err)
	resp.ID = msg.ID
	return resp
}

func (msg *Message) response(result any) *Message {
	enc, err := json.Marshal(result)
	if err != nil {
		// TODO: wrap with 'internal server error'
		return msg.errorResponse(err)
	}
	return &Message{Version: Version, ID: msg.ID, Result: enc}
}

func errorMessage(err error) *Message {
	msg := &Message{Version: Version, ID: null, Error: &jsonError{
		Code:    defaultErrorCode,
		Message: err.Error(),
	}}
	if e := Error(nil); errors.As(err, &e) {
		msg.Error.Code = e.ErrorCode()
	}
	if e := DataError(nil); errors.As(err, &e) {
		msg.Error.Data = e.ErrorData()
	}
	return msg
}

type jsonError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (err *jsonError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("json-rpc error %d", err.Code)
	}
	return err.Message
}

func (err *jsonError) ErrorCode() int {
	return err.Code
}

func (err *jsonError) ErrorData() any {
	return err.Data
}

// Conn is a subset of the methods of net.Conn which are sufficient for ServerCodec.
type Conn interface {
	io.ReadWriteCloser
	SetWriteDeadline(time.Time) error
}

type deadlineCloser interface {
	io.Closer
	SetWriteDeadline(time.Time) error
}

// ConnRemoteAddr wraps the RemoteAddr operation, which returns a description
// of the peer address of a connection. If a Conn also implements ConnRemoteAddr, this
// description is used in log messages.
type ConnRemoteAddr interface {
	RemoteAddr() string
}

// jsonCodec reads and writes JSON-RPC messages to the underlying connection. It also has
// support for parsing arguments and serializing (result) objects.
type jsonCodec struct {
	remote  string
	closer  sync.Once         // close closed channel once
	closeCh chan any          // closed on Close
	decode  func(v any) error // decoder to allow multiple transports
	encMu   sync.Mutex        // guards the encoder
	encode  func(v any) error // encoder to allow multiple transports
	conn    deadlineCloser
}

// NewFuncCodec creates a codec which uses the given functions to read and write. If conn
// implements ConnRemoteAddr, log messages will use it to include the remote address of
// the connection.
func NewFuncCodec(conn deadlineCloser, encode, decode func(v any) error) ServerCodec {
	codec := &jsonCodec{
		closeCh: make(chan any),
		encode:  encode,
		decode:  decode,
		conn:    conn,
	}
	if ra, ok := conn.(ConnRemoteAddr); ok {
		codec.remote = ra.RemoteAddr()
	}
	return codec
}

// NewCodec creates a codec on the given connection. If conn implements ConnRemoteAddr, log
// messages will use it to include the remote address of the connection.
func NewCodec(conn Conn) ServerCodec {
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	dec.UseNumber()
	return NewFuncCodec(conn, enc.Encode, dec.Decode)
}

func (c *jsonCodec) RemoteAddr() string {
	return c.remote
}

func (c *jsonCodec) Read() (messages []*Message, batch bool, err error) {
	// Decode the next JSON object in the input stream.
	// This verifies basic syntax, etc.
	var rawmsg json.RawMessage
	if err := c.decode(&rawmsg); err != nil {
		return nil, false, err
	}
	messages, batch = parseMessage(rawmsg)
	for i, msg := range messages {
		if msg == nil {
			// Message is JSON 'null'. Replace with zero value so it
			// will be treated like any other invalid message.
			messages[i] = new(Message)
		}
	}
	return messages, batch, nil
}

func (c *jsonCodec) WriteJSON(ctx context.Context, v any) error {
	c.encMu.Lock()
	defer c.encMu.Unlock()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultWriteTimeout)
	}
	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return c.encode(v)
}

func (c *jsonCodec) Close() {
	c.closer.Do(func() {
		close(c.closeCh)
		_ = c.conn.Close()
	})
}

// Closed returns a channel which will be closed when Close is called
func (c *jsonCodec) Closed() <-chan any {
	return c.closeCh
}

// parseMessage parses raw bytes as a (batch of) JSON-RPC message(s). There are no error
// checks in this function because the raw message has already been syntax-checked when it
// is called. Any non-JSON-RPC messages in the input return the zero value of
// Message.
func parseMessage(raw json.RawMessage) ([]*Message, bool) {
	if !isBatch(raw) {
		msgs := []*Message{{}}
		_ = json.Unmarshal(raw, &msgs[0])
		return msgs, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	_, _ = dec.Token() // skip '['
	var msgs []*Message
	for dec.More() {
		msgs = append(msgs, new(Message))
		_ = dec.Decode(&msgs[len(msgs)-1])
	}
	return msgs, true
}

// isBatch returns true when the first non-whitespace characters is '['
func isBatch(raw json.RawMessage) bool {
	for _, c := range raw {
		// skip insignificant whitespace (http://www.ietf.org/rfc/rfc4627.txt)
		if c == 0x20 || c == 0x09 || c == 0x0a || c == 0x0d {
			continue
		}
		return c == '['
	}
	return false
}

// parsePositionalArguments tries to parse the given args to an array of values with the
// given types. It returns the parsed values or an error when the args could not be
// parsed. Missing optional arguments are returned as reflect.Zero values.
func parsePositionalArguments(rawArgs json.RawMessage, types []reflect.Type) ([]reflect.Value, error) {
	dec := json.NewDecoder(bytes.NewReader(rawArgs))
	var args []reflect.Value
	tok, err := dec.Token()
	switch {
	case errors.Is(err, io.EOF) || tok == nil && err == nil:
		// "params" is optional and may be empty. Also allow "params":null even though it's
		// not in the spec because our own client used to send it.
	case err != nil:
		return nil, err
	case tok == json.Delim('['):
		// Read argument array.
		if args, err = parseArgumentArray(dec, types); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("non-array args")
	}
	// Set any missing args to nil.
	for i := len(args); i < len(types); i++ {
		if types[i].Kind() != reflect.Ptr {
			return nil, fmt.Errorf("missing value for required argument %d", i)
		}
		args = append(args, reflect.Zero(types[i]))
	}
	return args, nil
}

func parseArgumentArray(dec *json.Decoder, types []reflect.Type) ([]reflect.Value, error) {
	args := make([]reflect.Value, 0, len(types))
	for i := 0; dec.More(); i++ {
		if i >= len(types) {
			return args, fmt.Errorf("too many arguments, want at most %d", len(types))
		}
		argval := reflect.New(types[i])
		if err := dec.Decode(argval.Interface()); err != nil {
			return args, fmt.Errorf("invalid argument %d: %w", i, err)
		}
		if argval.IsNil() && types[i].Kind() != reflect.Ptr {
			return args, fmt.Errorf("missing value for required argument %d", i)
		}
		args = append(args, argval.Elem())
	}
	// Read end of args array.
	_, err := dec.Token()
	return args, err
}
