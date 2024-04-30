package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
)

// handler handles JSON-RPC messages. There is one handler per connection. Note that
// handler is not safe for concurrent use. Message handling never blocks indefinitely
// because RPCs are processed on background goroutines launched by handler.
//
// The entry points for incoming messages are:
//
//	h.handleMsg(message)
//	h.handleBatch(message)
//
// Outgoing calls use the requestOp struct. Register the request before sending it
// on the connection:
//
//	op := &requestOp{ids: ...}
//	h.addRequestOp(op)
//
// Now send the request, then wait for the reply to be delivered through handleMsg:
//
//	if err := op.wait(...); err != nil {
//	    h.removeRequestOp(op) // timeout, etc.
//	}
type handler struct {
	reg        *serviceRegistry
	respWait   map[string]*requestOp // active client requests
	callWG     sync.WaitGroup        // pending call goroutines
	rootCtx    context.Context       // canceled by close()
	cancelRoot func()                // cancel function for rootCtx
	conn       jsonWriter            // where responses will be sent
	logger     *zerolog.Logger

	allowList     AllowList // a list of explicitly allowed methods, if empty -- everything is allowed
	forbiddenList ForbiddenList

	maxBatchConcurrency uint
	traceRequests       bool

	//slow requests
	slowLogThreshold time.Duration
	slowLogBlacklist []string
}

type callProc struct {
	ctx context.Context
}

func HandleError(err error, stream *jsoniter.Stream) {
	if err != nil {
		stream.WriteObjectField("error")
		stream.WriteObjectStart()
		stream.WriteObjectField("code")
		ec, ok := err.(Error)
		if ok {
			stream.WriteInt(ec.ErrorCode())
		} else {
			stream.WriteInt(defaultErrorCode)
		}
		stream.WriteMore()
		stream.WriteObjectField("message")
		stream.WriteString(err.Error())
		de, ok := err.(DataError)
		if ok {
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
}

func newHandler(connCtx context.Context, conn jsonWriter, reg *serviceRegistry, allowList AllowList, maxBatchConcurrency uint, traceRequests bool, logger *zerolog.Logger, rpcSlowLogThreshold time.Duration) *handler {
	rootCtx, cancelRoot := context.WithCancel(connCtx)
	forbiddenList := newForbiddenList()

	h := &handler{
		reg:           reg,
		conn:          conn,
		respWait:      make(map[string]*requestOp),
		rootCtx:       rootCtx,
		cancelRoot:    cancelRoot,
		logger:        logger,
		allowList:     allowList,
		forbiddenList: forbiddenList,

		maxBatchConcurrency: maxBatchConcurrency,
		traceRequests:       traceRequests,

		slowLogThreshold: rpcSlowLogThreshold,
		slowLogBlacklist: rpccfg.SlowLogBlackList,
	}

	return h
}

func (h *handler) isRpcMethodNeedsCheck(method string) bool {
	for _, m := range h.slowLogBlacklist {
		if m == method {
			return false
		}
	}
	return true
}

// handleBatch executes all messages in a batch and returns the responses.
func (h *handler) handleBatch(msgs []*jsonrpcMessage) {
	// Emit error response for empty batches:
	if len(msgs) == 0 {
		h.startCallProc(func(cp *callProc) {
			_ = h.conn.WriteJSON(cp.ctx, errorMessage(&invalidRequestError{"empty batch"}))
		})
		return
	}

	// Handle non-call messages first:
	calls := make([]*jsonrpcMessage, 0, len(msgs))
	for _, msg := range msgs {
		if handled := h.handleImmediate(msg); !handled {
			calls = append(calls, msg)
		}
	}
	if len(calls) == 0 {
		return
	}
	// Process calls on a goroutine because they may block indefinitely:
	h.startCallProc(func(cp *callProc) {
		// All goroutines will place results right to this array. Because requests order must match reply orders.
		answersWithNils := make([]interface{}, len(msgs))
		// Bounded parallelism pattern explanation https://blog.golang.org/pipelines#TOC_9.
		boundedConcurrency := make(chan struct{}, h.maxBatchConcurrency)
		defer close(boundedConcurrency)
		wg := sync.WaitGroup{}
		wg.Add(len(msgs))
		for i := range calls {
			boundedConcurrency <- struct{}{}
			go func(i int) {
				defer func() {
					wg.Done()
					<-boundedConcurrency
				}()

				select {
				case <-cp.ctx.Done():
					return
				default:
				}

				buf := bytes.NewBuffer(nil)
				stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 4096)
				if res := h.handleCallMsg(cp, calls[i], stream); res != nil {
					answersWithNils[i] = res
				}
				_ = stream.Flush()
				if buf.Len() > 0 && answersWithNils[i] == nil {
					answersWithNils[i] = json.RawMessage(buf.Bytes())
				}
			}(i)
		}
		wg.Wait()
		answers := make([]interface{}, 0, len(msgs))
		for _, answer := range answersWithNils {
			if answer != nil {
				answers = append(answers, answer)
			}
		}
		if len(answers) > 0 {
			_ = h.conn.WriteJSON(cp.ctx, answers)
		}
	})
}

// handleMsg handles a single message.
func (h *handler) handleMsg(msg *jsonrpcMessage) {
	if ok := h.handleImmediate(msg); ok {
		return
	}
	h.startCallProc(func(cp *callProc) {
		stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 4096)
		answer := h.handleCallMsg(cp, msg, stream)
		if answer != nil {
			buffer, _ := json.Marshal(answer)
			_, _ = stream.Write(buffer)
		}
		_ = h.conn.WriteJSON(cp.ctx, json.RawMessage(stream.Buffer()))
	})
}

// close cancels all requests except for inflightReq and waits for
// call goroutines to shut down.
func (h *handler) close(err error, inflightReq *requestOp) {
	h.cancelAllRequests(err, inflightReq)
	h.callWG.Wait()
	h.cancelRoot()
}

// addRequestOp registers a request operation.
func (h *handler) addRequestOp(op *requestOp) {
	for _, id := range op.ids {
		h.respWait[string(id)] = op
	}
}

// removeRequestOps stops waiting for the given request IDs.
func (h *handler) removeRequestOp(op *requestOp) {
	for _, id := range op.ids {
		delete(h.respWait, string(id))
	}
}

// cancelAllRequests unblocks and removes pending requests.
func (h *handler) cancelAllRequests(err error, inflightReq *requestOp) {
	didClose := make(map[*requestOp]bool)
	if inflightReq != nil {
		didClose[inflightReq] = true
	}

	for id, op := range h.respWait {
		// Remove the op so that later calls will not close op.resp again.
		delete(h.respWait, id)

		if !didClose[op] {
			op.err = err
			close(op.resp)
			didClose[op] = true
		}
	}
}

// startCallProc runs fn in a new goroutine and starts tracking it in the h.calls wait group.
func (h *handler) startCallProc(fn func(*callProc)) {
	h.callWG.Add(1)
	go func() {
		ctx, cancel := context.WithCancel(h.rootCtx)
		defer h.callWG.Done()
		defer cancel()
		fn(&callProc{ctx: ctx})
	}()
}

// handleImmediate executes non-call messages. It returns false if the message is a
// call or requires a reply.
func (h *handler) handleImmediate(msg *jsonrpcMessage) bool {
	start := time.Now()
	switch {
	case msg.isResponse():
		h.handleResponse(msg)
		h.logger.Trace().Str("reqid", idForLog(msg.ID).String()).Str("t", time.Since(start).String()).Msg("[rpc] handled response")
		return true
	default:
		return false
	}
}

// handleResponse processes method call responses.
func (h *handler) handleResponse(msg *jsonrpcMessage) {
	op := h.respWait[string(msg.ID)]
	if op == nil {
		h.logger.Trace().Str("reqid", idForLog(msg.ID).String()).Msg("[rpc] unsolicited response")
		return
	}
	delete(h.respWait, string(msg.ID))
	// For normal responses, just forward the reply to Call.
	op.resp <- msg
}

// handleCallMsg executes a call message and returns the answer.
func (h *handler) handleCallMsg(ctx *callProc, msg *jsonrpcMessage, stream *jsoniter.Stream) *jsonrpcMessage {
	start := time.Now()
	switch {
	case msg.isNotification():
		h.handleCall(ctx, msg, stream)
		if h.traceRequests {
			h.logger.Info().Str("t", time.Since(start).String()).Str("method", msg.Method).Str("params", string(msg.Params)).Msg("[rpc] served")
		} else {
			h.logger.Trace().Str("t", time.Since(start).String()).Str("method", msg.Method).Str("params", string(msg.Params)).Msg("[rpc] served")
		}
		return nil
	case msg.isCall():
		var doSlowLog bool
		if h.slowLogThreshold > 0 {
			doSlowLog = h.isRpcMethodNeedsCheck(msg.Method)
			if doSlowLog {
				slowTimer := time.AfterFunc(h.slowLogThreshold, func() {
					h.logger.Info().Str("t", time.Since(start).String()).Str("method", msg.Method).Str("params", string(msg.Params)).Msg("[rpc.slow] running")
				})
				defer slowTimer.Stop()
			}
		}

		resp := h.handleCall(ctx, msg, stream)

		if doSlowLog {
			requestDuration := time.Since(start)
			if requestDuration > h.slowLogThreshold {
				h.logger.Info().Str("t", time.Since(start).String()).Str("method", msg.Method).Str("params", string(msg.Params)).Msg("[rpc.slow] finished")
			}
		}

		if resp != nil && resp.Error != nil {
			h.logger.Info().
				Str("t", time.Since(start).String()).
				Str("method", msg.Method).
				Str("reqid", idForLog(msg.ID).String()).
				Str("err", resp.Error.Message).
				Msg("[rpc] served")
		}
		var event *zerolog.Event
		if h.traceRequests {
			event = h.logger.Info()
		} else {
			event = h.logger.Trace()
		}

		event.
			Str("t", time.Since(start).String()).
			Str("method", msg.Method).
			Str("reqid", idForLog(msg.ID).String()).
			Str("params", string(msg.Params)).
			Msg("Served")

		return resp
	case msg.hasValidID():
		return msg.errorResponse(&invalidRequestError{"invalid request"})
	default:
		return errorMessage(&invalidRequestError{"invalid request"})
	}
}

func (h *handler) isMethodAllowedByGranularControl(method string) bool {
	_, isForbidden := h.forbiddenList[method]
	if len(h.allowList) == 0 {
		return !isForbidden
	}

	_, ok := h.allowList[method]
	return ok
}

// handleCall processes method calls.
func (h *handler) handleCall(cp *callProc, msg *jsonrpcMessage, stream *jsoniter.Stream) *jsonrpcMessage {
	var callb *callback
	if h.isMethodAllowedByGranularControl(msg.Method) {
		callb = h.reg.callback(msg.Method)
	}
	if callb == nil {
		return msg.errorResponse(&methodNotFoundError{method: msg.Method})
	}
	args, err := parsePositionalArguments(msg.Params, callb.argTypes)
	if err != nil {
		return msg.errorResponse(&InvalidParamsError{err.Error()})
	}
	return h.runMethod(cp.ctx, msg, callb, args, stream)
}

// runMethod runs the Go callback for an RPC method.
func (h *handler) runMethod(ctx context.Context, msg *jsonrpcMessage, callb *callback, args []reflect.Value, stream *jsoniter.Stream) *jsonrpcMessage {
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
	stream.Flush()
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
