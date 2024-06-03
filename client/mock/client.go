package mock

import "encoding/json"

type (
	callDelegate func(method string, params []interface{}) (json.RawMessage, error)
)

type MockClient struct {
	CallFn callDelegate
}

func (m *MockClient) Call(method string, params []interface{}) (json.RawMessage, error) {
	if m.CallFn != nil {
		return m.CallFn(method, params)
	}

	return nil, nil
}
