package mock

import (
	"math/big"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
)

type MockClient struct {
	CallResult map[string]any
	Block      *jsonrpc.RPCBlock
	Str        *string
	Code       *types.Code
	Hash       *common.Hash
	InMessage  *jsonrpc.RPCInMessage
	Receipt    *jsonrpc.RPCReceipt
	Counter    *uint64
	Seqno      *types.Seqno
	Balance    *big.Int
	Err        error
}

func (m *MockClient) Call(method string, params ...any) (map[string]any, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.CallResult != nil {
		return m.CallResult, nil
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
