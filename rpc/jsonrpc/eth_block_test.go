package jsonrpc

import (
	"context"
	"strconv"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBlockByNumber(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer database.Close()

	api := NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), database, common.NewLogger("Test", false))
	_, err = api.GetBlockByNumber(context.Background(), transport.EarliestBlockNumber, false)
	require.EqualError(t, err, "not implemented")

	_, err = api.GetBlockByNumber(context.Background(), transport.LatestBlockNumber, false)
	require.EqualError(t, err, "Key not found")

	block := types.Block{
		Id:                 0,
		PrevBlock:          common.EmptyHash,
		SmartContractsRoot: common.EmptyHash,
		TransactionsRoot:   common.EmptyHash,
	}
	blockHash := block.Hash()

	err = database.Put(db.LastBlockTable, []byte(strconv.Itoa(0)), blockHash.Bytes())
	require.NoError(t, err)

	tx, err := database.CreateTx(ctx)
	defer tx.Rollback()
	require.NoError(t, err)

	err = db.WriteBlock(tx, &block)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	data, err := api.GetBlockByNumber(context.Background(), transport.LatestBlockNumber, false)
	require.NoError(t, err)
	assert.Equal(t, common.EmptyHash, data["parentHash"])
	assert.Equal(t, blockHash, data["hash"])
}

func TestGetBlockByHash(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer database.Close()

	api := NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), database, common.NewLogger("Test", false))

	block := types.Block{
		Id:                 0,
		PrevBlock:          common.EmptyHash,
		SmartContractsRoot: common.EmptyHash,
		TransactionsRoot:   common.EmptyHash,
	}
	blockHash := block.Hash()

	err = database.Put(db.LastBlockTable, []byte(strconv.Itoa(0)), blockHash.Bytes())
	require.NoError(t, err)

	tx, err := database.CreateTx(ctx)
	defer tx.Rollback()
	require.NoError(t, err)

	err = db.WriteBlock(tx, &block)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	data, err := api.GetBlockByHash(context.Background(), blockHash, false)
	require.NoError(t, err)
	assert.IsType(t, map[string]any{}, data)
	assert.Equal(t, common.EmptyHash, data["parentHash"])
	assert.Equal(t, blockHash, data["hash"])
}

func TestGetBlockTransactionCountByHash(t *testing.T) {
	ctx := context.Background()

	blockHash := common.HexToHash("0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef")

	db, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer db.Close()

	api := NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), db, common.NewLogger("Test", false))
	_, err = api.GetBlockTransactionCountByHash(ctx, blockHash)
	require.EqualError(t, err, "not implemented")
}

func TestGetBlockTransactionCountByNumber(t *testing.T) {
	ctx := context.Background()

	db, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer db.Close()

	api := NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), db, common.NewLogger("Test", false))
	_, err = api.GetBlockTransactionCountByNumber(ctx, transport.LatestBlockNumber)
	require.EqualError(t, err, "not implemented")
}
