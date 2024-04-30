package transport

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

func newTestServer(logger *zerolog.Logger) *Server {
	server := NewServer(50, false /* traceRequests */, false /* debugSingleRequests */, logger, 100)
	if err := server.RegisterName("test", new(testService)); err != nil {
		panic(err)
	}
	return server
}

type testService struct{}

type echoArgs struct {
	S string
}

type echoResult struct {
	String string
	Int    int
	Args   *echoArgs
}

type testError struct{}

func (testError) Error() string          { return "testError" }
func (testError) ErrorCode() int         { return 444 }
func (testError) ErrorData() interface{} { return "testError data" }

func (s *testService) NoArgsRets() {}

func (s *testService) Echo(str string, i int, args *echoArgs) echoResult {
	return echoResult{str, i, args}
}

func (s *testService) EchoWithCtx(ctx context.Context, str string, i int, args *echoArgs) echoResult {
	return echoResult{str, i, args}
}

func (s *testService) Sleep(ctx context.Context, duration time.Duration) {
	time.Sleep(duration)
}

func (s *testService) Block(ctx context.Context) error {
	<-ctx.Done()
	return errors.New("context canceled in testservice_block")
}

func (s *testService) Rets() (string, error) {
	return "", nil
}

func (s *testService) ReturnError() error {
	return testError{}
}

// largeRespService generates arbitrary-size JSON responses.
type largeRespService struct {
	length int
}

func (x largeRespService) LargeResp() string {
	return strings.Repeat("x", x.length)
}
