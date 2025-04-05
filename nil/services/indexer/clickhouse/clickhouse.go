package clickhouse

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	indexerdriver "github.com/NilFoundation/nil/nil/services/indexer/driver"
	indexertypes "github.com/NilFoundation/nil/nil/services/indexer/types"
)

type ClickhouseDriver struct {
	conn       driver.Conn
	insertConn driver.Conn
	options    clickhouse.Options
}

func (d *ClickhouseDriver) FetchBlock(ctx context.Context, id types.ShardId, number types.BlockNumber) (*types.Block, error) {
	row := d.conn.QueryRow(ctx, `
		SELECT binary
		FROM blocks
		WHERE shard_id = $1 AND id = $2
		LIMIT 1
	`, id, number)

	var binary []byte
	if err := row.Scan(&binary); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan block binary: %w", err)
	}

	var block types.Block
	if err := block.UnmarshalSSZ(binary); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}

func (d *ClickhouseDriver) FetchLatestProcessedBlockId(ctx context.Context, id types.ShardId) (*types.BlockNumber, error) {
	blockNum, err := d.blockIdFromRow(d.conn.QueryRow(ctx, `
		SELECT id
		FROM blocks
		WHERE shard_id = $1
		ORDER BY id DESC
		LIMIT 1
	`, id))
	if err != nil {
		return nil, err
	}
	return &blockNum, nil
}

func (d *ClickhouseDriver) FetchAddressActions(
	ctx context.Context,
	address types.Address,
	timestamp db.Timestamp,
) ([]indexertypes.AddressAction, error) {
	rows, err := d.conn.Query(context.Background(), `
		SELECT
			t.hash,
			t.from,
			t.to,
			t.value as amount,
			t.timestamp,
			t.block_id,
			t.success,
			t.binary
		FROM transactions t
		WHERE (t.from = $1 OR t.to = $1) AND t.timestamp >= $2
		ORDER BY t.timestamp ASC
	`, address, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var actions []indexertypes.AddressAction
	for rows.Next() {
		var action indexertypes.AddressAction
		var success bool
		var txnBinary []byte
		if err := rows.Scan(
			&action.Hash,
			&action.From,
			&action.To,
			&action.Amount,
			&action.Timestamp,
			&action.BlockId,
			&success,
			&txnBinary,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		// Set the status based on success
		if success {
			action.Status = indexertypes.Success
		} else {
			action.Status = indexertypes.Failed
		}

		// Set the action type based on the address relationship
		if action.From == address {
			if action.Amount.Uint64() == 0 {
				action.Type = indexertypes.SmartContractCall
			} else {
				action.Type = indexertypes.SendEth
			}
		} else {
			action.Type = indexertypes.ReceiveEth
		}

		actions = append(actions, action)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return actions, nil
}

func (d *ClickhouseDriver) IndexBlocks(ctx context.Context, ids []*indexerdriver.BlockWithShardId) error {
	return d.ExportBlocks(ctx, ids)
}

// I saw this trick. dunno should I use it here too
var (
	_ indexerdriver.IndexerDriver = &ClickhouseDriver{}
)

// extend types.Block with binary field
type BlockWithBinary struct {
	types.Block
	Binary   []byte        `ch:"binary"`
	Hash     common.Hash   `ch:"hash"`
	ShardId  types.ShardId `ch:"shard_id"`
	InTxnNum uint64        `ch:"in_txn_num"`
}

type TransactionWithBinary struct {
	types.Transaction
	Success           bool                   `ch:"success"`
	Status            string                 `ch:"status"`
	GasUsed           types.Gas              `ch:"gas_used"`
	BlockId           types.BlockNumber      `ch:"block_id"`
	BlockHash         common.Hash            `ch:"block_hash"`
	Binary            []byte                 `ch:"binary"`
	ReceiptBinary     []byte                 `ch:"receipt_binary"`
	Hash              common.Hash            `ch:"hash"`
	ShardId           types.ShardId          `ch:"shard_id"`
	TransactionIndex  types.TransactionIndex `ch:"transaction_index"`
	Outgoing          bool                   `ch:"outgoing"`
	Timestamp         uint64                 `ch:"timestamp"`
	ParentTransaction common.Hash            `ch:"parent_transaction"`
	ErrorMessage      string                 `ch:"error_message"`
	FailedPc          uint32                 `ch:"failed_pc"`
}

func NewTransactionWithBinary(
	transaction *types.Transaction,
	transactionBinary sszx.SSZEncodedData,
	receipt *types.Receipt,
	receiptBinary sszx.SSZEncodedData,
	block *types.BlockWithExtractedData,
	idx types.TransactionIndex,
	shardId types.ShardId,
) *TransactionWithBinary {
	hash := transaction.Hash()
	res := &TransactionWithBinary{
		Transaction:      *transaction,
		Binary:           transactionBinary,
		BlockId:          block.Id,
		BlockHash:        block.Hash(shardId),
		Hash:             hash,
		ShardId:          shardId,
		TransactionIndex: idx,
		Timestamp:        block.Timestamp,
		ErrorMessage:     block.Errors[hash],
	}
	if receipt != nil {
		res.Success = receipt.Success
		res.GasUsed = receipt.GasUsed
		res.ReceiptBinary = receiptBinary
		res.FailedPc = receipt.FailedPc
	}
	return res
}

type LogWithBinary struct {
	TransactionHash common.Hash   `ch:"transaction_hash"`
	Binary          []byte        `ch:"binary"`
	Address         types.Address `ch:"address"`
	TopicsCount     uint8         `ch:"topics_count"`
	Topic1          common.Hash   `ch:"topic1"`
	Topic2          common.Hash   `ch:"topic2"`
	Topic3          common.Hash   `ch:"topic3"`
	Topic4          common.Hash   `ch:"topic4"`
	Data            []byte        `ch:"data"`
}

func NewLogWithBinary(log *types.Log, binary []byte, receipt *types.Receipt) *LogWithBinary {
	res := &LogWithBinary{
		TransactionHash: receipt.TxnHash,
		Binary:          binary,
		Address:         log.Address,
		TopicsCount:     uint8(len(log.Topics)),
		Data:            log.Data,
	}
	for i, topic := range log.Topics {
		switch i {
		case 0:
			res.Topic1 = topic
		case 1:
			res.Topic2 = topic
		case 2:
			res.Topic3 = topic
		case 3:
			res.Topic4 = topic
		}
	}
	return res
}

func NewClickhouseDriver(ctx context.Context, endpoint, login, password, database string) (*ClickhouseDriver, error) {
	if err := common.CreateClickHouseDbIfNotExists(ctx, database, login, password, endpoint); err != nil {
		return nil, err
	}
	// Create connection to Clickhouse
	connectionOptions := clickhouse.Options{
		Auth: clickhouse.Auth{
			Username: login,
			Password: password,
			Database: database,
		},
		Addr: []string{endpoint},
	}
	conn, err := clickhouse.Open(&connectionOptions)
	if err != nil {
		return nil, err
	}

	insertConn, err := clickhouse.Open(&connectionOptions)
	if err != nil {
		return nil, err
	}
	return &ClickhouseDriver{
		conn:       conn,
		insertConn: insertConn,
		options:    connectionOptions,
	}, nil
}

func (d *ClickhouseDriver) Reconnect() error {
	var err error
	d.conn, err = clickhouse.Open(&d.options)
	if err != nil {
		return err
	}

	d.insertConn, err = clickhouse.Open(&d.options)
	return err
}

func (d *ClickhouseDriver) SetupScheme(ctx context.Context, params indexerdriver.SetupParams) error {
	version, err := readVersion(ctx, d.conn)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if bytes.Equal(version[:], params.Version[:]) {
		return nil
	}

	if !params.AllowDbDrop {
		return fmt.Errorf("version mismatch: blockchain %x, indexer %x", params.Version, version)
	}

	if version.Empty() {
		logger.Info().Msg("Database is empty. Recreating...")
	} else {
		logger.Info().Msgf("Version mismatch: blockchain %x, indexer %x. Dropping database...", params.Version, version)
	}

	for table := range getTableScheme() {
		if err := d.conn.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", d.options.Auth.Database, table)); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	return setupSchemes(ctx, d.conn)
}

// blockIdFromRow returns block id from the first column of the row.
// If the row is empty, returns -1, so you can use the (result + 1) as the next block id.
func (d *ClickhouseDriver) blockIdFromRow(row driver.Row) (types.BlockNumber, error) {
	var blockNumber uint64
	if err := row.Scan(&blockNumber); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.InvalidBlockNumber, nil
		}
		return 0, err
	}
	return types.BlockNumber(blockNumber), nil
}

func (d *ClickhouseDriver) FetchEarliestAbsentBlockId(ctx context.Context, shardId types.ShardId) (types.BlockNumber, error) {
	// We look for `a.id` such that there is no `a.id+1` in the table.
	// Left (outer) join will set `b.id` to 0 in that case.
	id, err := d.blockIdFromRow(d.conn.QueryRow(ctx, `
		SELECT a.id
		FROM blocks AS a
			LEFT JOIN blocks AS b
				ON a.id + 1 = b.id AND a.shard_id = b.shard_id
		WHERE a.shard_id = $1 AND b.id == 0
		ORDER BY a.id ASC
		LIMIT 1
	`, shardId))
	if err != nil {
		return 0, err
	}
	return id + 1, nil
}

type blockWithSSZ struct {
	decoded    *indexerdriver.BlockWithShardId
	sszEncoded *types.RawBlockWithExtractedData
}

type receiptWithSSZ struct {
	decoded    *types.Receipt
	sszEncoded sszx.SSZEncodedData
}

func (d *ClickhouseDriver) ExportBlocks(ctx context.Context, blocksToExport []*indexerdriver.BlockWithShardId) error {
	blocks := make([]blockWithSSZ, len(blocksToExport))
	for blockIndex, block := range blocksToExport {
		sszEncodedBlock, err := block.EncodeSSZ()
		if err != nil {
			return err
		}
		blocks[blockIndex] = blockWithSSZ{decoded: block, sszEncoded: sszEncodedBlock}
	}

	if err := exportTransactionsAndLogs(ctx, d.insertConn, blocks); err != nil {
		return err
	}

	blockBatch, err := d.insertConn.PrepareBatch(ctx, "INSERT INTO blocks")
	if err != nil {
		return err
	}

	for _, block := range blocks {
		binary, blockErr := block.decoded.MarshalSSZ()
		if blockErr != nil {
			return blockErr
		}
		binaryBlockExtended := &BlockWithBinary{
			Block:    *block.decoded.Block,
			Binary:   binary,
			ShardId:  block.decoded.ShardId,
			Hash:     block.decoded.Hash(block.decoded.ShardId),
			InTxnNum: uint64(len(block.decoded.InTransactions)),
		}
		blockErr = blockBatch.AppendStruct(binaryBlockExtended)
		if blockErr != nil {
			return fmt.Errorf("failed to append block to batch: %w", blockErr)
		}
	}

	err = blockBatch.Send()
	if err != nil {
		return err
	}

	return nil
}

func exportTransactionsAndLogs(ctx context.Context, conn driver.Conn, blocks []blockWithSSZ) error {
	transactionBatch, err := conn.PrepareBatch(ctx, "INSERT INTO transactions")
	if err != nil {
		return err
	}

	for _, block := range blocks {
		parentIndex := make([]common.Hash, len(block.decoded.OutTransactions))
		if len(block.decoded.InTransactions) != len(block.decoded.Receipts) {
			return fmt.Errorf("block in txs count mismatch: %d != %d", len(block.decoded.InTransactions),
				len(block.decoded.Receipts))
		}
		for inTxnIndex, transaction := range block.decoded.InTransactions {
			hash := transaction.Hash()
			receipt := receiptWithSSZ{
				decoded:    block.decoded.Receipts[inTxnIndex],
				sszEncoded: block.sszEncoded.Receipts[inTxnIndex],
			}
			if receipt.decoded.TxnHash != hash {
				return fmt.Errorf("receipt's transaction hash mismatch: %s != %s", receipt.decoded.TxnHash, hash)
			}
			if receipt.decoded.OutTxnIndex+receipt.decoded.OutTxnNum > uint32(len(parentIndex)) {
				return fmt.Errorf(
					"output txs range is out of bound: [index=%d, num=%d], block out txs count: %d, block: %d.%d",
					receipt.decoded.OutTxnIndex, receipt.decoded.OutTxnNum, len(parentIndex), block.decoded.ShardId,
					block.decoded.Id)
			}
			for i := receipt.decoded.OutTxnIndex; i < receipt.decoded.OutTxnIndex+receipt.decoded.OutTxnNum; i++ {
				parentIndex[i] = hash
			}
			mb := NewTransactionWithBinary(
				transaction,
				block.sszEncoded.InTransactions[inTxnIndex],
				receipt.decoded,
				receipt.sszEncoded,
				block.decoded.BlockWithExtractedData,
				types.TransactionIndex(inTxnIndex),
				block.decoded.ShardId)
			if err := transactionBatch.AppendStruct(mb); err != nil {
				return fmt.Errorf("failed to append transaction to batch: %w", err)
			}
		}
		for outTransactionIndex, transaction := range block.decoded.OutTransactions {
			mb := NewTransactionWithBinary(
				transaction,
				block.sszEncoded.OutTransactions[outTransactionIndex],
				nil,
				nil,
				block.decoded.BlockWithExtractedData,
				types.TransactionIndex(outTransactionIndex),
				block.decoded.ShardId)
			mb.Outgoing = true
			mb.ParentTransaction = parentIndex[outTransactionIndex]
			if err := transactionBatch.AppendStruct(mb); err != nil {
				return fmt.Errorf("failed to append transaction to batch: %w", err)
			}
		}
	}

	err = transactionBatch.Send()
	if err != nil {
		return fmt.Errorf("failed to send transactions batch: %w", err)
	}

	logBatch, err := conn.PrepareBatch(ctx, "INSERT INTO logs")
	if err != nil {
		return fmt.Errorf("failed to prepare log batch: %w", err)
	}

	for _, block := range blocks {
		for _, receipt := range block.decoded.Receipts {
			for _, log := range receipt.Logs {
				binary, logErr := log.MarshalSSZ()
				if logErr != nil {
					return logErr
				}
				if err := logBatch.AppendStruct(NewLogWithBinary(log, binary, receipt)); err != nil {
					return fmt.Errorf("failed to append log to batch: %w", err)
				}
			}
		}
	}

	if err = logBatch.Send(); err != nil {
		return fmt.Errorf("failed to send logs batch: %w", err)
	}
	return nil
}

func (d *ClickhouseDriver) HaveBlock(ctx context.Context, id types.ShardId, number types.BlockNumber) (bool, error) {
	row := d.conn.QueryRow(ctx, `
		SELECT count()
		FROM blocks
		WHERE shard_id = $1 AND id = $2
	`, id, number)
	var count uint64
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	check.PanicIfNot(count <= 1)
	return count > 0, nil
}

func (d *ClickhouseDriver) FetchNextPresentBlockId(ctx context.Context, shardId types.ShardId, number types.BlockNumber) (types.BlockNumber, error) {
	return d.blockIdFromRow(d.conn.QueryRow(ctx, `
		SELECT id
		FROM blocks
		WHERE shard_id = $1 AND id > $2
		ORDER BY id ASC
		LIMIT 1
	`, shardId, number))
}
