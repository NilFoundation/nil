package rpctest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

type Request struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	Id      int    `json:"id"`
}

func NewRequest(method string, params ...any) *Request {
	return &Request{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		Id:      1,
	}
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Response[R any] struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  R      `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
	Id      int    `json:"id"`
}

const (
	getBlockByHash                   = "eth_getBlockByHash"
	getBlockByNumber                 = "eth_getBlockByNumber"
	sendRawTransaction               = "eth_sendRawTransaction"
	getInMessageByHash               = "eth_getInMessageByHash"
	getInMessageReceipt              = "eth_getInMessageReceipt"
	getTransactionCount              = "eth_getTransactionCount"
	getBlockTransactionCountByNumber = "eth_getBlockTransactionCountByNumber"
	getBlockTransactionCountByHash   = "eth_getBlockTransactionCountByHash"
)

func makeRequest[R any](port int, data *Request) (*Response[R], error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post("http://127.0.0.1:"+strconv.Itoa(port), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response Response[R]
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
