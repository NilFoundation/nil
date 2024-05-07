package rpctest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type Request struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      int           `json:"id"`
}

type Response struct {
	Jsonrpc string                 `json:"jsonrpc"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   map[string]interface{} `json:"error,omitempty"`
	Id      int                    `json:"id"`
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
	go startRpcServer(suite.context, 1, suite.T().TempDir()+"/test.db")
	time.Sleep(time.Second) // To be sure that server is started
}

func (suite *SuiteRpc) TearDownSuite() {
	suite.cancel()
}

func (suite *SuiteRpc) TestRpcBasic() {
	request := Request{
		Jsonrpc: "2.0",
		Method:  "eth_getBlockByNumber",
		Params:  []interface{}{"0x1b4", false},
		Id:      1,
	}

	resp, err := makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0)
	suite.Equal("not implemented", resp.Error["message"])

	request.Method = "eth_getBlockByHash"
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0)
	suite.Equal("not implemented", resp.Error["message"])

	request.Method = "eth_getBlockTransactionCountByNumber"
	request.Params = []interface{}{"0x1b4"}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0)
	suite.Equal("not implemented", resp.Error["message"])

	request.Method = "eth_getBlockTransactionCountByHash"
	request.Params = []interface{}{"0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef"}
	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0)
	suite.Equal("not implemented", resp.Error["message"])
}

func (suite *SuiteRpc) TestRpcApiModules() {
	request := Request{
		Jsonrpc: "2.0",
		Method:  "rpc_modules",
		Params:  []interface{}{},
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
		Params:  []interface{}{},
		Id:      1,
	}

	resp, err := makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32601), resp.Error["code"], 0)
	suite.Equal("the method eth_doesntExists does not exist/is not available", resp.Error["message"])

	request = Request{
		Jsonrpc: "2.0",
		Method:  "eth_getBlockByNumber",
		Params:  []interface{}{},
		Id:      1,
	}

	resp, err = makeRequest(&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32602), resp.Error["code"], 0)
	suite.Equal("missing value for required argument 0", resp.Error["message"])
}

func TestSuiteRpc(t *testing.T) {
	suite.Run(t, new(SuiteRpc))
}
