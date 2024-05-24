package rpctest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/suite"
)

const (
	getBlockByHash   = "eth_getBlockByHash"
	getBlockByNumber = "eth_getBlockByNumber"
)

type Request struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	Id      int    `json:"id"`
}

type Response struct {
	Jsonrpc string         `json:"jsonrpc"`
	Result  map[string]any `json:"result,omitempty"`
	Error   map[string]any `json:"error,omitempty"`
	Id      int            `json:"id"`
}

type SuiteRpc struct {
	suite.Suite
	context context.Context
	cancel  context.CancelFunc
}

func makeRequest(data *Request) (*Response, error) {
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

	var response Response
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (suite *SuiteRpc) SetupSuite() {
	suite.context, suite.cancel = context.WithCancel(context.Background())

	dbOpts := db.BadgerDBOptions{Path: suite.T().TempDir() + "/test.db", DiscardRatio: 0.5, GcFrequency: time.Hour, AllowDrop: false}
	badger, err := db.NewBadgerDb(dbOpts.Path)
	suite.Require().NoError(err)

	go nilservice.Run(suite.context, 2, badger, dbOpts)
	time.Sleep(time.Second) // To be sure that server is started
}

func (suite *SuiteRpc) TearDownSuite() {
	suite.cancel()
}

func (suite *SuiteRpc) TestRpcBasic() {
	request := Request{
		Jsonrpc: "2.0",
		Method:  getBlockByNumber,
		Params:  []any{types.MasterShardId, "0x1b4", false},
		Id:      1,
	}

	resp, err := makeRequest(&request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Require().Nil(resp.Result)

	request.Method = "eth_getBlockTransactionCountByNumber"
	request.Params = []any{types.MasterShardId, "0x1b4"}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0)
	suite.Equal("not implemented", resp.Error["message"])

	request.Method = "eth_getBlockTransactionCountByHash"
	request.Params = []any{types.MasterShardId, "0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef"}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0)
	suite.Equal("not implemented", resp.Error["message"])

	request.Method = getBlockByHash
	request.Params = []any{types.MasterShardId, "0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef", false}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Require().Nil(resp.Result)

	request.Method = getBlockByNumber
	request.Params = []any{types.MasterShardId, "earliest", false}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error)
	suite.Require().NotNil(resp.Result)

	request.Method = getBlockByNumber
	request.Params = []any{types.MasterShardId, "latest", false}
	latestResp, err := makeRequest(&request)
	suite.Require().NoError(err)
	suite.Require().Nil(latestResp.Error["code"])
	suite.Require().NotNil(latestResp.Result["hash"])

	request.Method = getBlockByHash
	hash, ok := latestResp.Result["hash"].(string)
	suite.Require().True(ok)
	request.Params = []any{types.MasterShardId, hash, false}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Require().Equal(latestResp.Result, resp.Result)

	request.Method = "eth_getMessageByHash"
	request.Params = []any{types.MasterShardId, "0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef"}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Require().Nil(resp.Result)
}

func (suite *SuiteRpc) TestRpcApiModules() {
	request := Request{
		Jsonrpc: "2.0",
		Method:  "rpc_modules",
		Params:  []any{},
		Id:      1,
	}

	resp, err := makeRequest(&request)
	suite.Require().NoError(err)
	suite.Equal("1.0", resp.Result["eth"])
	suite.Equal("1.0", resp.Result["rpc"])
}

func (suite *SuiteRpc) TestRpcError() {
	request := Request{
		Jsonrpc: "2.0",
		Method:  "eth_doesntExists",
		Params:  []any{},
		Id:      1,
	}

	resp, err := makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32601), resp.Error["code"], 0)
	suite.Equal("the method eth_doesntExists does not exist/is not available", resp.Error["message"])

	request = Request{
		Jsonrpc: "2.0",
		Method:  getBlockByNumber,
		Params:  []any{},
		Id:      1,
	}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32602), resp.Error["code"], 0)
	suite.Equal("missing value for required argument 0", resp.Error["message"])

	request.Method = getBlockByNumber
	request.Params = []any{1 << 40}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32602), resp.Error["code"], 0)
	suite.Equal(
		"invalid argument 0: json: cannot unmarshal number 1099511627776 into Go value of type uint32",
		resp.Error["message"])

	request.Method = getBlockByNumber
	request.Params = []any{types.MasterShardId}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32602), resp.Error["code"], 0)
	suite.Equal("missing value for required argument 1", resp.Error["message"])

	request.Method = getBlockByHash
	request.Params = []any{types.MasterShardId, "0x1b4", false}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0.5)
	suite.Equal("invalid argument 1: hex string of odd length", resp.Error["message"])

	request.Method = getBlockByHash
	request.Params = []any{types.MasterShardId, "latest"}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0.5)
	suite.Equal("invalid argument 1: hex string without 0x prefix", resp.Error["message"])
}

func (suite *SuiteRpc) TestRpcDebugModules() {
	request := Request{
		Jsonrpc: "2.0",
		Method:  "debug_getBlockByNumber",
		Params:  []any{types.MasterShardId, "latest"},
	}

	resp, err := makeRequest(&request)
	suite.Require().NoError(err)

	suite.Require().Contains(resp.Result, "number")
	suite.Require().Contains(resp.Result, "hash")
	suite.Require().Contains(resp.Result, "content")

	sliceContent, ok := resp.Result["content"].(string)
	suite.Require().True(ok)
	// check if the string starts with 0x prefix
	suite.Require().Equal("0x", sliceContent[:2])
	// print resp to see the result
	suite.T().Logf("resp: %v", resp)
}

func TestSuiteRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpc))
}
