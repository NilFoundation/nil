package transport

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"
)

// handler handles JSON-RPC messages. There is one handler per connection. Note that
// handler is not safe for concurrent use. Message handling never blocks indefinitely
// because RPCs are processed on background goroutines launched by handler.
//
// The entry points for incoming messages are:
//
//	h.handleMsg(message)
type handler struct {
	reg        *serviceRegistry
	callWG     sync.WaitGroup  // pending call goroutines
	rootCtx    context.Context // canceled by close()
	cancelRoot func()          // cancel function for rootCtx
	conn       JsonWriter      // where responses will be sent
	logger     zerolog.Logger

	traceRequests bool

	// slow requests
	slowLogThreshold time.Duration
	slowLogBlacklist []string
}

func HandleError(err error, stream *jsoniter.Stream) {
	if err == nil {
		return
	}

	stream.WriteObjectField("error")
	stream.WriteObjectStart()
	stream.WriteObjectField("code")
	if ec := Error(nil); errors.As(err, &ec) {
		stream.WriteInt(ec.ErrorCode())
	} else {
		stream.WriteInt(defaultErrorCode)
	}
	stream.WriteMore()
	stream.WriteObjectField("message")
	stream.WriteString(err.Error())

	if de := DataError(nil); errors.As(err, &de) {
		stream.WriteMore()
		stream.WriteObjectField("data")
		data, derr := json.Marshal(de.ErrorData())
		if derr == nil {
			if _, err := stream.Write(data); err != nil {
				stream.WriteNil()
			}
		} else {
			stream.WriteString(derr.Error())
		}
	}
	stream.WriteObjectEnd()
}

func newHandler(connCtx context.Context, conn JsonWriter, reg *serviceRegistry, traceRequests bool, logger zerolog.Logger, rpcSlowLogThreshold time.Duration) *handler {
	rootCtx, cancelRoot := context.WithCancel(connCtx)

	h := &handler{
		reg:        reg,
		conn:       conn,
		rootCtx:    rootCtx,
		cancelRoot: cancelRoot,
		logger:     logger,

		traceRequests: traceRequests,

		slowLogThreshold: rpcSlowLogThreshold,
		slowLogBlacklist: rpccfg.SlowLogBlackList,
	}

	return h
}

func (h *handler) log(lvl zerolog.Level, msg *Message, logMsg string, duration time.Duration) {
	l := h.logger.WithLevel(lvl).
		Stringer(logging.FieldReqId, idForLog(msg.ID)).
		Str(logging.FieldRpcMethod, msg.Method).
		Str(logging.FieldRpcParams, string(msg.Params))
	if duration > 0 {
		l = l.Dur(logging.FieldDuration, duration)
	}
	l.Msg(logMsg)
}

func (h *handler) requestLoggingLevel() zerolog.Level {
	if h.traceRequests {
		return zerolog.InfoLevel
	}
	return zerolog.TraceLevel
}

func (h *handler) isRpcMethodNeedsCheck(method string) bool {
	for _, m := range h.slowLogBlacklist {
		if m == method {
			return false
		}
	}
	return true
}

// handleMsg handles a single message.
func (h *handler) handleMsg(msg *Message) {
	h.startCallProc(func() {
		stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 4096)
		answer := h.handleCallMsg(h.rootCtx, msg, stream)
		if answer != nil {
			buffer, _ := json.Marshal(answer)
			_, _ = stream.Write(buffer)
		}
		_ = h.conn.WriteJSON(h.rootCtx, json.RawMessage(stream.Buffer()))
	})
}

// close cancels all requests except for inflightReq and waits for
// call goroutines to shut down.
func (h *handler) close() {
	h.cancelRoot()
	h.callWG.Wait()
}

// startCallProc runs fn in a new goroutine and starts tracking it in the h.calls wait group.
func (h *handler) startCallProc(fn func()) {
	h.callWG.Add(1)
	go func() {
		defer h.callWG.Done()
		fn()
	}()
}

// handleCallMsg executes a call message and returns the answer.
func (h *handler) handleCallMsg(ctx context.Context, msg *Message, stream *jsoniter.Stream) *Message {
	start := time.Now()
	switch {
	case msg.isCall():
		var doSlowLog bool
		if h.slowLogThreshold > 0 {
			doSlowLog = h.isRpcMethodNeedsCheck(msg.Method)
			if doSlowLog {
				slowTimer := time.AfterFunc(h.slowLogThreshold, func() {
					h.log(zerolog.InfoLevel, msg, "Slow running request", time.Since(start))
				})
				defer slowTimer.Stop()
			}
		}

		resp := h.handleCall(ctx, msg, stream)
		requestDuration := time.Since(start)

		if doSlowLog {
			if requestDuration > h.slowLogThreshold {
				h.log(zerolog.InfoLevel, msg, "Slow request finished.", requestDuration)
			}
		}

		if resp != nil && resp.Error != nil {
			h.log(zerolog.InfoLevel, msg, "Served with error: "+resp.Error.Message, requestDuration)
		}

		h.log(h.requestLoggingLevel(), msg, "Served.", requestDuration)

		return resp
	case msg.hasValidID():
		return msg.errorResponse(&invalidRequestError{"invalid request"})
	default:
		return errorMessage(&invalidRequestError{"invalid request"})
	}
}

// handleCall processes method calls.
func (h *handler) handleCall(ctx context.Context, msg *Message, stream *jsoniter.Stream) *Message {
	callb := h.reg.callback(msg.Method)
	if callb == nil {
		return msg.errorResponse(&methodNotFoundError{method: msg.Method})
	}
	args, err := parsePositionalArguments(msg.Params, callb.argTypes)
	if err != nil {
		return msg.errorResponse(&InvalidParamsError{err.Error()})
	}
	return h.runMethod(ctx, msg, callb, args, stream)
}

// runMethod runs the Go callback for an RPC method.
func (h *handler) runMethod(ctx context.Context, msg *Message, callb *callback, args []reflect.Value, stream *jsoniter.Stream) *Message {
	if !callb.streamable {
		result, err := callb.call(ctx, msg.Method, args, stream)
		if err != nil {
			return msg.errorResponse(err)
		}
		return msg.response(result)
	}

	stream.WriteObjectStart()
	stream.WriteObjectField("jsonrpc")
	stream.WriteString("2.0")
	stream.WriteMore()
	if msg.ID != nil {
		stream.WriteObjectField("id")
		_, _ = stream.Write(msg.ID)
		stream.WriteMore()
	}
	stream.WriteObjectField("result")
	_, err := callb.call(ctx, msg.Method, args, stream)
	if err != nil {
		writeNilIfNotPresent(stream)
		stream.WriteMore()
		HandleError(err, stream)
	}
	stream.WriteObjectEnd()
	_ = stream.Flush()
	return nil
}

var nullAsBytes = []byte{110, 117, 108, 108}

// there are many avenues that could lead to an error being handled in runMethod, so we need to check
// if nil has already been written to the stream before writing it again here
func writeNilIfNotPresent(stream *jsoniter.Stream) {
	if stream == nil {
		return
	}
	b := stream.Buffer()
	hasNil := true
	if len(b) >= 4 {
		b = b[len(b)-4:]
		for i, v := range nullAsBytes {
			if v != b[i] {
				hasNil = false
				break
			}
		}
	} else {
		hasNil = false
	}
	if !hasNil {
		stream.WriteNil()
	}
}

type idForLog json.RawMessage

func (id idForLog) String() string {
	if s, err := strconv.Unquote(string(id)); err == nil {
		return s
	}
	return string(id)
}
