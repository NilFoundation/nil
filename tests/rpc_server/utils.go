package rpctest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
)

type Request struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	Id      int    `json:"id"`
}

type Response[R any] struct {
	Jsonrpc string         `json:"jsonrpc"`
	Result  R              `json:"result,omitempty"`
	Error   map[string]any `json:"error,omitempty"`
	Id      int            `json:"id"`
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

func makeRequest[R any](data *Request) (*Response[R], error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post("http://127.0.0.1:8529", "application/json", bytes.NewBuffer(jsonData))
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

func transactionCount(shardId types.ShardId, addr common.Address, blk string) (uint64, error) {
	request := &Request{
		Jsonrpc: "2.0",
		Method:  getTransactionCount,
		Params:  []any{shardId, addr.Hex(), blk},
		Id:      1,
	}

	resp, err := makeRequest[string](request)
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(resp.Result, 0, 64)
}
