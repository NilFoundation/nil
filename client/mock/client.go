package mock

import (
	"crypto/ecdsa"
	"encoding/json"
	"math/big"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
)

type MockClient struct {
	RawCallResult json.RawMessage
	Block         *jsonrpc.RPCBlock
	Str           *string
	Code          *types.Code
	Hash          *common.Hash
	InMessage     *jsonrpc.RPCInMessage
	Receipt       *jsonrpc.RPCReceipt
	Counter       *uint64
	Seqno         *types.Seqno
	Balance       *big.Int
	Err           error
}

func (m *MockClient) RawCall(method string, params ...any) (json.RawMessage, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.RawCallResult != nil {
		return m.RawCallResult, nil
	}
	return nil, nil
}

func (m *MockClient) GetCode(addr types.Address, blockNrOrHash any) (types.Code, error) {
	if m.Err != nil {
		return types.Code{}, m.Err
	}
	if m.Code != nil {
		return *m.Code, nil
	}
	return types.Code{}, nil
}

func (m *MockClient) GetBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Block != nil {
		return m.Block, nil
	}
	return nil, nil
}

func (m *MockClient) SendMessage(msg *types.ExternalMessage) (common.Hash, error) {
	if m.Err != nil {
		return common.EmptyHash, m.Err
	}
	if m.Hash != nil {
		return *m.Hash, nil
	}
	return common.EmptyHash, nil
}

func (m *MockClient) SendRawTransaction(data []byte) (common.Hash, error) {
	if m.Err != nil {
		return common.EmptyHash, m.Err
	}
	if m.Hash != nil {
		return *m.Hash, nil
	}
	return common.EmptyHash, nil
}

func (m *MockClient) GetInMessageByHash(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCInMessage, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.InMessage != nil {
		return m.InMessage, nil
	}
	return nil, nil
}

func (m *MockClient) GetInMessageReceipt(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCReceipt, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Receipt != nil {
		return m.Receipt, nil
	}
	return nil, nil
}

func (m *MockClient) GetTransactionCount(address types.Address, blockNrOrHash any) (types.Seqno, error) {
	if m.Err != nil {
		return 0, m.Err
	}
	if m.Seqno != nil {
		return *m.Seqno, nil
	}
	return 0, nil
}

func (m *MockClient) GetBlockTransactionCount(shardId types.ShardId, blockId any) (uint64, error) {
	if m.Err != nil {
		return 0, m.Err
	}
	if m.Counter != nil {
		return *m.Counter, nil
	}
	return 0, nil
}

func (m *MockClient) GetBalance(address types.Address, blockNrOrHash any) (*big.Int, error) {
	if m.Err != nil {
		return big.NewInt(0), m.Err
	}
	return m.Balance, nil
}

func (m *MockClient) DeployContract(shardId types.ShardId, address types.Address, bytecode types.Code, pk *ecdsa.PrivateKey) (common.Hash, types.Address, error) {
	hash := common.EmptyHash
	addr := types.EmptyAddress

	if m.Err != nil {
		return hash, addr, m.Err
	}

	if m.Hash != nil {
		hash = *m.Hash
	}

	return hash, addr, nil
}

func (m *MockClient) SendMessageViaWallet(address types.Address, bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey) (common.Hash, error) {
	hash := common.EmptyHash
	if m.Hash != nil {
		hash = *m.Hash
	}
	return hash, m.Err
}

func (m *MockClient) Call(args *jsonrpc.CallArgs) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	if m.Str != nil {
		return *m.Str, nil
	}
	return "", nil
}
