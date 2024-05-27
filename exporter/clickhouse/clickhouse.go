package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/exporter"
)

type ClickhouseDriver struct {
	conn       driver.Conn
	insertConn driver.Conn
}

// I saw this trick. dunno should I use it here too
var (
	_ exporter.ExportDriver = &ClickhouseDriver{}
)

var tableSchemeCache = make(map[string][]string)

func init() {
	intiSchemeCache()
}

func intiSchemeCache() {
	scheme, err := ReflectSchemeToClickhouse(&types.Block{})
	if err != nil {
		panic(err)
	}
	tableSchemeCache["blocks"] = scheme
}

func NewClickhouseDriver(_ context.Context, endpoint, login, password, database string) (*ClickhouseDriver, error) {
	// Create connection to Clickhouse
	conn, err := clickhouse.Open(&clickhouse.Options{
		Auth: clickhouse.Auth{
			Username: login,
			Password: password,
			Database: database,
		},
		Addr: []string{endpoint},
	})
	if err != nil {
		return nil, err
	}

	insertConn, err := clickhouse.Open(&clickhouse.Options{
		Auth: clickhouse.Auth{
			Username: login,
			Password: password,
			Database: database,
		},
		Addr: []string{endpoint},
	})
	if err != nil {
		return nil, err
	}
	return &ClickhouseDriver{
		conn:       conn,
		insertConn: insertConn,
	}, nil
}

func (d *ClickhouseDriver) SetupScheme(ctx context.Context) error {
	return setupSchemeForClickhouse(ctx, d.conn)
}

func rowToBlock(rows driver.Rows) (*types.Block, error) {
	var binary []uint8
	if err := rows.Scan(&binary); err != nil {
		return nil, err
	}
	var block types.Block
	if err := block.UnmarshalSSZ(binary); err != nil {
		return nil, err
	}
	return &block, nil
}

func (d *ClickhouseDriver) FetchLatestProcessedBlock(ctx context.Context, shardId types.ShardId) (*types.Block, bool, error) {
	rows, err := d.conn.Query(ctx, `SELECT blocks.binary
from blocks
where shard_id = $1
order by blocks.Id desc
limit 1`, shardId)
	defer func(rows driver.Rows) {
		_ = rows.Close()
	}(rows)
	if err != nil {
		return nil, false, err
	}
	if !rows.Next() {
		return nil, false, nil
	}
	block, err := rowToBlock(rows)
	if err != nil {
		return nil, false, err
	}
	return block, true, nil
}

func (d *ClickhouseDriver) FetchEarliestAbsentBlock(ctx context.Context, shardId types.ShardId) (types.BlockNumber, bool, error) {
	// join in clickhouse return default value on outer join
	rows, err := d.conn.Query(ctx, `SELECT a.Id + 1
from blocks as a
         left outer join blocks as b on a.Id + 1 = b.Id and a.shard_id = b.shard_id
where a.shard_id = $1 and a.Id > b.Id
order by a.Id asc
`, shardId)
	defer func(rows driver.Rows) {
		_ = rows.Close()
	}(rows)
	if err != nil {
		return 0, false, err
	}
	if !rows.Next() {
		return 0, false, nil
	}
	var blockNumber uint64
	if err = rows.Scan(&blockNumber); err != nil {
		return 0, false, err
	}

	return types.BlockNumber(blockNumber), true, nil
}

func setupSchemeForClickhouse(ctx context.Context, conn driver.Conn) error {
	// Create table for blocks
	fields, ok := tableSchemeCache["blocks"]
	if !ok {
		return errors.New("scheme for blocks not found")
	}

	scheme := strings.Join(fields, ",\n")

	err := conn.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS blocks (
		    binary Array(UInt8),
		    hash FixedString(32),
		    shard_id UInt32,
			%s
		) ENGINE = ReplacingMergeTree()
		PRIMARY KEY (shard_id, hash)
		order by (shard_id, hash)
	`, scheme))
	if err != nil {
		return fmt.Errorf("failed to create table blocks: %w", err)
	}

	return nil
}

// extend types.Block with binary field
type BlockWithBinary struct {
	types.Block
	Binary  []byte        `ch:"binary"`
	Hash    []byte        `ch:"hash"`
	ShardId types.ShardId `ch:"shard_id"`
}

func (d *ClickhouseDriver) ExportBlocks(ctx context.Context, msgs []*exporter.BlockMsg) error {
	batch, err := d.insertConn.PrepareBatch(ctx, "INSERT INTO blocks")
	if err != nil {
		return err
	}

	for _, block := range msgs {
		binary, blockErr := block.Block.MarshalSSZ()
		if blockErr != nil {
			return blockErr
		}
		binaryBlockExtended := &BlockWithBinary{
			Block:   *block.Block,
			Binary:  binary,
			ShardId: block.Shard,
			Hash:    block.Block.Hash().Bytes(),
		}
		blockErr = batch.AppendStruct(binaryBlockExtended)
		if blockErr != nil {
			return blockErr
		}
	}

	err = batch.Send()
	if err != nil {
		return err
	}

	return nil
}

func (d *ClickhouseDriver) FetchBlock(ctx context.Context, id types.ShardId, number types.BlockNumber) (*types.Block, bool, error) {
	rows, err := d.conn.Query(ctx, "SELECT binary FROM blocks WHERE shard_id = $1 AND Id = $2", id, number)
	if err != nil {
		return nil, false, err
	}
	if !rows.Next() {
		return nil, false, nil
	}
	block, err := rowToBlock(rows)
	if err != nil {
		return nil, false, err
	}
	return block, true, nil
}

func (d *ClickhouseDriver) FetchNextPresentBlock(ctx context.Context, shardId types.ShardId, number types.BlockNumber) (types.BlockNumber, bool, error) {
	rows, err := d.conn.Query(ctx, `SELECT blocks.Id
from blocks 
where shard_id = $1 and Id > $2 order by Id asc`, shardId, number)
	defer func(rows driver.Rows) {
		_ = rows.Close()
	}(rows)
	if err != nil {
		return 0, false, err
	}
	if !rows.Next() {
		return 0, false, nil
	}
	var blockNumber uint64
	if err := rows.Scan(&blockNumber); err != nil {
		return 0, false, err
	}
	return types.BlockNumber(blockNumber), true, nil
}
