package transport

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestClientRequest(t *testing.T) {
	logger := common.NewLogger("Test", false)
	server := newTestServer(logger)
	defer server.Stop()
	client := DialInProc(server, logger)
	defer client.Close()

	var resp echoResult
	if err := client.Call(&resp, "test_echo", "hello", 10, &echoArgs{"world"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resp, echoResult{"hello", 10, &echoArgs{"world"}}) {
		t.Errorf("incorrect result %#v", resp)
	}
}

func TestClientResponseType(t *testing.T) {
	logger := common.NewLogger("Test", false)
	server := newTestServer(logger)
	defer server.Stop()
	client := DialInProc(server, logger)
	defer client.Close()

	if err := client.Call(nil, "test_echo", "hello", 10, &echoArgs{"world"}); err != nil {
		t.Errorf("Passing nil as result should be fine, but got an error: %v", err)
	}
	var resultVar echoResult
	// Note: passing the var, not a ref
	err := client.Call(resultVar, "test_echo", "hello", 10, &echoArgs{"world"})
	if err == nil {
		t.Error("Passing a var as result should be an error")
	}
}

// This test checks that server-returned errors with code and data come out of Client.Call.
func TestClientErrorData(t *testing.T) {
	logger := common.NewLogger("Test", false)
	server := newTestServer(logger)
	defer server.Stop()
	client := DialInProc(server, logger)
	defer client.Close()

	var resp interface{}
	err := client.Call(&resp, "test_returnError")
	if err == nil {
		t.Fatal("expected error")
	}

	// Check code.
	if e, ok := err.(Error); !ok {
		t.Fatalf("client did not return rpc.Error, got %#v", e)
	} else if e.ErrorCode() != (testError{}.ErrorCode()) {
		t.Fatalf("wrong error code %d, want %d", e.ErrorCode(), testError{}.ErrorCode())
	}
	// Check data.
	if e, ok := err.(DataError); !ok {
		t.Fatalf("client did not return rpc.DataError, got %#v", e)
	} else if e.ErrorData() != (testError{}.ErrorData()) {
		t.Fatalf("wrong error data %#v, want %#v", e.ErrorData(), testError{}.ErrorData())
	}
}

func TestClientCancelHTTP(t *testing.T) {
	testClientCancel("http", t, common.NewLogger("Test", false))
}

// This test checks that requests made through CallContext can be canceled by canceling
// the context.
func testClientCancel(transport string, t *testing.T, logger *zerolog.Logger) {
	// These tests take a lot of time, run them all at once.
	// You probably want to run with -parallel 1 or comment out
	// the call to t.Parallel if you enable the logging.
	t.Parallel()

	server := newTestServer(logger)
	defer server.Stop()

	// What we want to achieve is that the context gets canceled
	// at various stages of request processing. The interesting cases
	// are:
	//  - cancel during dial
	//  - cancel while performing a HTTP request
	//  - cancel while waiting for a response
	//
	// To trigger those, the times are chosen such that connections
	// are killed within the deadline for every other call (maxKillTimeout
	// is 2x maxCancelTimeout).
	//
	// Once a connection is dead, there is a fair chance it won't connect
	// successfully because the accept is delayed by 1s.
	maxContextCancelTimeout := 300 * time.Millisecond
	fl := &flakeyListener{
		maxAcceptDelay: 1 * time.Second,
		maxKillTimeout: 600 * time.Millisecond,
	}

	var client *Client
	switch transport {
	case "http":
		c, hs := httpTestClient(server, transport, fl)
		defer hs.Close()
		client = c
	default:
		panic("unknown transport: " + transport)
	}

	// The actual test starts here.
	var (
		wg       sync.WaitGroup
		nreqs    = 10
		ncallers = 10
	)
	caller := func(index int) {
		defer wg.Done()
		for i := 0; i < nreqs; i++ {
			var (
				ctx     context.Context
				cancel  func()
				timeout = time.Duration(rand.Int63n(int64(maxContextCancelTimeout)))
			)
			if index < ncallers/2 {
				// For half of the callers, create a context without deadline
				// and cancel it later.
				ctx, cancel = context.WithCancel(context.Background())
				time.AfterFunc(timeout, cancel)
			} else {
				// For the other half, create a context with a deadline instead. This is
				// different because the context deadline is used to set the socket write
				// deadline.
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
			}

			// Now perform a call with the context.
			// The key thing here is that no call will ever complete successfully.
			err := client.CallContext(ctx, nil, "test_block")
			if err == nil {
				_, hasDeadline := ctx.Deadline()
				t.Errorf("no error for call with %v wait time (deadline: %v)", timeout, hasDeadline)
				// default:
				// 	t.Logf("got expected error with %v wait time: %v", timeout, err)
			}
			cancel()
		}
	}
	wg.Add(ncallers)
	for i := 0; i < ncallers; i++ {
		go caller(i)
	}
	wg.Wait()
}

func TestClientSetHeader(t *testing.T) {
	logger := common.NewLogger("Test", false)
	srv := newTestServer(logger)
	httpsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.ServeHTTP(w, r)
	}))
	defer httpsrv.Close()
	defer srv.Stop()

	client, err := Dial(httpsrv.URL, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
}

func TestClientHTTP(t *testing.T) {
	logger := common.NewLogger("Test", false)
	server := newTestServer(logger)
	defer server.Stop()

	client, hs := httpTestClient(server, "http", nil)
	defer hs.Close()
	defer client.Close()

	// Launch concurrent requests.
	var (
		results    = make([]echoResult, 100)
		errc       = make(chan error, len(results))
		wantResult = echoResult{"a", 1, new(echoArgs)}
	)
	defer client.Close()
	for i := range results {
		i := i
		go func() {
			errc <- client.Call(&results[i], "test_echo", wantResult.String, wantResult.Int, wantResult.Args)
		}()
	}

	// Wait for all of them to complete.
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for i := range results {
		select {
		case err := <-errc:
			if err != nil {
				t.Fatal(err)
			}
		case <-timeout.C:
			t.Fatalf("timeout (got %d/%d) results)", i+1, len(results))
		}
	}

	// Check results.
	for i := range results {
		if !reflect.DeepEqual(results[i], wantResult) {
			t.Errorf("result %d mismatch: got %#v, want %#v", i, results[i], wantResult)
		}
	}
}

func httpTestClient(srv *Server, transport string, fl *flakeyListener) (*Client, *httptest.Server) {
	logger := common.NewLogger("Test", false)
	// Create the HTTP server.
	var hs *httptest.Server
	switch transport {
	case "http":
		hs = httptest.NewUnstartedServer(srv)
	default:
		panic("unknown HTTP transport: " + transport)
	}
	// Wrap the listener if required.
	if fl != nil {
		fl.Listener = hs.Listener
		hs.Listener = fl
	}
	// Connect the client.
	hs.Start()
	client, err := Dial(transport+"://"+hs.Listener.Addr().String(), logger)
	if err != nil {
		panic(err)
	}
	return client, hs
}

// flakeyListener kills accepted connections after a random timeout.
type flakeyListener struct {
	net.Listener
	maxKillTimeout time.Duration
	maxAcceptDelay time.Duration
}

func (l *flakeyListener) Accept() (net.Conn, error) {
	delay := time.Duration(rand.Int63n(int64(l.maxAcceptDelay)))
	time.Sleep(delay)

	c, err := l.Listener.Accept()
	if err == nil {
		timeout := time.Duration(rand.Int63n(int64(l.maxKillTimeout)))
		time.AfterFunc(timeout, func() {
			log.Trace().Msg(fmt.Sprintf("killing conn %v after %v", c.LocalAddr(), timeout))
			c.Close()
		})
	}
	return c, err
}
