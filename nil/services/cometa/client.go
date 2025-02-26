package cometa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type Client struct {
	requestId  atomic.Uint64
	endpoint   string
	httpClient http.Client
}

func NewClient(url string) *Client {
	httpc, endpoint := rpc_client.NewHttpClient(url)
	return &Client{
		httpClient: httpc,
		endpoint:   endpoint,
	}
}

func (c *Client) IsValid() bool {
	return len(c.endpoint) > 0
}

func (c *Client) sendRequest(method string, params []any) (json.RawMessage, error) {
	request := make(map[string]any)
	request["jsonrpc"] = "2.0"
	request["method"] = method
	request["params"] = params
	request["id"] = c.requestId.Load()
	c.requestId.Add(1)

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "cometa/"+version.GetGitRevCount())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, body)
	}

	var rpcResponse map[string]json.RawMessage
	if err := json.Unmarshal(body, &rpcResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if errorMsg, ok := rpcResponse["error"]; ok {
		return nil, fmt.Errorf("rpc error: %s", errorMsg)
	}

	return rpcResponse["result"], nil
}

func (c *Client) GetContract(address types.Address) (*ContractData, error) {
	response, err := c.sendRequest("cometa_getContract", []any{address})
	if err != nil {
		return nil, err
	}
	var contract ContractData
	if err := json.Unmarshal(response, &contract); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %w", err)
	}
	return &contract, nil
}

func (c *Client) GetContractFields(address types.Address, fieldNames []string) ([]any, error) {
	response, err := c.sendRequest("cometa_getContractFields", []any{address, fieldNames})
	if err != nil {
		return nil, err
	}
	var res []any
	if err = json.Unmarshal(response, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %w", err)
	}
	return res, nil
}

func (c *Client) GetLocation(address types.Address, pc uint64) (*Location, error) {
	response, err := c.sendRequest("cometa_getLocation", []any{address, pc})
	if err != nil {
		return nil, err
	}
	var loc Location
	if err := json.Unmarshal(response, &loc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %w", err)
	}
	return &loc, nil
}

func (c *Client) GetAbi(address types.Address) (abi.ABI, error) {
	response, err := c.sendRequest("cometa_getAbi", []any{address})
	if err != nil {
		return abi.ABI{}, err
	}
	var str *string
	if err := json.Unmarshal(response, &str); err != nil {
		return abi.ABI{}, fmt.Errorf("failed to unmarshal abi: %w", err)
	}
	if str == nil {
		return abi.ABI{}, ErrAbiNotFound
	}
	return abi.JSON(strings.NewReader(*str))
}

func (c *Client) CompileContract(inputJsonFile string) (*ContractData, error) {
	inputJson, err := os.ReadFile(inputJsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read input json: %w", err)
	}
	var task CompilerTask
	if err := json.Unmarshal(inputJson, &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input json: %w", err)
	}
	task.BasePath = filepath.Dir(inputJsonFile)
	if err := task.Normalize(filepath.Dir(inputJsonFile)); err != nil {
		return nil, fmt.Errorf("failed to normalize compiler task: %w", err)
	}
	normInputJson, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input json: %w", err)
	}
	response, err := c.sendRequest("cometa_compileContract", []any{string(normInputJson)})
	if err != nil {
		return nil, err
	}
	var contractData ContractData
	if err := json.Unmarshal(response, &contractData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %w", err)
	}
	return &contractData, nil
}

func (c *Client) RegisterContractFromFile(inputJsonFile string, address types.Address) error {
	inputJson, err := os.ReadFile(inputJsonFile)
	if err != nil {
		return fmt.Errorf("failed to read input json: %w", err)
	}
	task, err := NewCompilerTask(string(inputJson))
	if err != nil {
		return fmt.Errorf("failed to read input json: %w", err)
	}
	if err = task.Normalize(filepath.Dir(inputJsonFile)); err != nil {
		return fmt.Errorf("failed to normalize compiler task: %w", err)
	}
	inputJson, err = json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal input json: %w", err)
	}

	_, err = c.sendRequest("cometa_registerContract", []any{string(inputJson), address})
	return err
}

func (c *Client) RegisterContract(inputJson string, address types.Address) error {
	_, err := c.sendRequest("cometa_registerContract", []any{inputJson, address})
	return err
}

func (c *Client) RegisterContractData(contractData *ContractData, address types.Address) error {
	_, err := c.sendRequest("cometa_registerContractData", []any{contractData, address})
	return err
}

func (c *Client) DecodeTransactionsCallData(transactions []TransactionInfo) ([]string, error) {
	response, err := c.sendRequest("cometa_decodeTransactionsCallData", []any{transactions})
	if err != nil {
		return nil, err
	}
	var res []string
	if err := json.Unmarshal(response, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}
	return res, err
}
