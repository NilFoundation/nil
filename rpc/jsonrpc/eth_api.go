package jsonrpc

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog"
)

// EthAPI is a collection of functions that are exposed in the
type EthAPI interface {
	// Block related
	GetBlockByNumber(ctx context.Context, number transport.BlockNumber, fullTx bool) (map[string]any, error)
	GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]any, error)
	GetBlockTransactionCountByNumber(ctx context.Context, blockNr transport.BlockNumber) (*hexutil.Uint, error)
	GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (*hexutil.Uint, error)

	// Message related
	GetMessageByHash(ctx context.Context, hash common.Hash) (*types.Message, error)

	// Account related
	GetBalance(ctx context.Context, address common.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Big, error)
	GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Uint64, error)
	GetCode(ctx context.Context, address common.Address, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error)
}

type BaseAPI struct {
	evmCallTimeout time.Duration
}

func NewBaseApi(evmCallTimeout time.Duration) *BaseAPI {
	return &BaseAPI{
		evmCallTimeout: evmCallTimeout,
	}
}

// APIImpl is implementation of the EthAPI interface based on remote Db access
type APIImpl struct {
	*BaseAPI

	db     db.DB
	logger *zerolog.Logger
}

// NewEthAPI returns APIImpl instance
func NewEthAPI(base *BaseAPI, db db.DB, logger *zerolog.Logger) *APIImpl {
	return &APIImpl{
		BaseAPI: base,
		db:      db,
		logger:  logger,
	}
}
