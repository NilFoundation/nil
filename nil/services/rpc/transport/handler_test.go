package transport

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestHandlerDoesNotDoubleWriteNull(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		params   []byte
		expected string
	}{
		"error_with_stream_write": {
			params:   []byte("[1]"),
			expected: `{"jsonrpc":"2.0","id":1,"result":null,"error":{"code":-32000,"message":"id 1"}}`,
		},
		"error_without_stream_write": {
			params:   []byte("[2]"),
			expected: `{"jsonrpc":"2.0","id":1,"result":null,"error":{"code":-32000,"message":"id 2"}}`,
		},
		"no_error": {
			params:   []byte("[3]"),
			expected: `{"jsonrpc":"2.0","id":1,"result":{}}`,
		},
		"err_with_valid_json": {
			params:   []byte("[4]"),
			expected: `{"jsonrpc":"2.0","id":1,"result":{"structLogs":[]},"error":{"code":-32000,"message":"id 4"}}`,
		},
	}

	for name, testParams := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			msg := Message{
				Version: "2.0",
				ID:      []byte{49},
				Method:  "test_test",
				Params:  testParams.params,
				Error:   nil,
				Result:  nil,
			}

			dummyFunc := func(id int, stream *jsoniter.Stream) error {
				if id == 1 {
					stream.WriteNil()
					return errors.New("id 1")
				}
				if id == 2 {
					return errors.New("id 2")
				}
				if id == 3 {
					stream.WriteEmptyObject()
					return nil
				}
				if id == 4 {
					stream.WriteObjectStart()
					stream.WriteObjectField("structLogs")
					stream.WriteEmptyArray()
					stream.WriteObjectEnd()
					return errors.New("id 4")
				}
				return nil
			}

			var arg1 int
			cb := &callback{
				fn:         reflect.ValueOf(dummyFunc),
				rcvr:       reflect.Value{},
				argTypes:   []reflect.Type{reflect.TypeOf(arg1)},
				hasCtx:     false,
				errPos:     0,
				streamable: true,
			}

			args, err := parsePositionalArguments((msg).Params, cb.argTypes)
			if err != nil {
				t.Fatal(err)
			}

			var buf bytes.Buffer
			stream := jsoniter.NewStream(jsoniter.ConfigDefault, &buf, 4096)

			h := handler{}
			h.runMethod(context.Background(), &msg, cb, args, stream)

			output := buf.String()
			assert.Equal(t, testParams.expected, output, "expected output should match")
		})
	}
}
