package clickhouse

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/iden3/go-iden3-crypto/keccak256"
	"github.com/rs/zerolog/log"
)

type ClickhouseDriver struct {
	conn driver.Conn
}

func NewClickhouseDriver(ctx context.Context, endpoint, login, password, database string) (*ClickhouseDriver, error) {
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

	rows, err := conn.Query(ctx, `SELECT 1`)
	if err != nil {
		return nil, err
	}
	log.Info().Msg("Clickhouse connection established")
	defer func(rows driver.Rows) {
		_ = rows.Close()
	}(rows)
	return &ClickhouseDriver{
		conn: conn,
	}, nil
}

func (d *ClickhouseDriver) SetupScheme(ctx context.Context) error {
	return setupSchemeForClickhouse(ctx, d.conn)
}

func (d *ClickhouseDriver) ExportBlock(ctx context.Context, block *types.Block) error {
	return exportBlocksToClickhouse(ctx, []*types.Block{block}, d.conn)
}

func (d *ClickhouseDriver) ExportBlocks(ctx context.Context, blocks []*types.Block) error {
	return exportBlocksToClickhouse(ctx, blocks, d.conn)
}

func (d *ClickhouseDriver) FetchLatestBlock(ctx context.Context) (*types.Block, error) {
	queryPart := "max(Id)"
	return fetchBlockFromPoint(ctx, d.conn, queryPart)
}

func (d *ClickhouseDriver) FetchEarlierPoint(ctx context.Context) (*types.Block, error) {
	queryPart := "min(Id)"
	return fetchBlockFromPoint(ctx, d.conn, queryPart)
}

func setupSchemeForClickhouse(ctx context.Context, conn driver.Conn) error {
	// Create table for blocks
	fields, err := ReflectSchemeToClickhouse(&types.Block{})
	if err != nil {
		return err
	}

	scheme := strings.Join(fields, ",\n")

	tableName, err := getTableNameForType(&types.Block{})
	if err != nil {
		return err
	}

	err = conn.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
		    binary Array(UInt8),
		    hash FixedString(32),
			%s
		) ENGINE = MergeTree()
		PRIMARY KEY (hash)
	`, tableName, scheme))
	if err != nil {
		log.Err(err).Msg("Failed to create table")
		return err
	}

	return nil
}

// extend types.Block with binary field
type BlockWithBinary struct {
	types.Block
	Binary []byte `ch:"binary"`
	Hash   []byte `ch:"hash"`
}

func exportBlocksToClickhouse(ctx context.Context, blocks []*types.Block, conn driver.Conn) error {
	// Export block to Clickhouse

	tableName, err := getTableNameForType(&types.Block{})
	if err != nil {
		return err
	}

	batch, err := conn.PrepareBatch(ctx, "INSERT INTO "+tableName)
	if err != nil {
		return err
	}

	for _, block := range blocks {
		binary, blockErr := block.MarshalSSZ()
		if blockErr != nil {
			return blockErr
		}
		binaryBlockExtended := &BlockWithBinary{
			Block:  *block,
			Binary: binary,
			Hash:   block.Hash().Bytes(),
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

func getTableNameForType(someType any) (string, error) {
	fields, err := ReflectSchemeToClickhouse(someType)
	if err != nil {
		return "", err
	}

	baseTableName := strings.ToLower(reflect.TypeOf(someType).Elem().Name())

	schemeHash := keccak256.Hash([]byte(strings.Join(fields, ",\n")))

	return fmt.Sprintf("%s_%s", baseTableName, hexutil.Encode(schemeHash)), nil
}

func fetchBlockFromPoint(ctx context.Context, conn driver.Conn, queryPart string) (*types.Block, error) {
	tableName, err := getTableNameForType(&types.Block{})
	if err != nil {
		return nil, err
	}

	// Fetch last point from Clickhouse
	query := fmt.Sprintf("SELECT %s as point FROM %s", queryPart, tableName)

	log.Info().Msgf("Query: %s", query)

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows driver.Rows) {
		_ = rows.Close()
	}(rows)

	if !rows.Next() {
		return nil, nil
	}
	var point uint64
	if err = rows.Scan(&point); err != nil {
		log.Info().Msg("Failed to scan point from Clickhouse")
		return nil, err
	}

	var binary []uint8
	if err = conn.QueryRow(ctx, fmt.Sprintf("SELECT binary FROM %s WHERE Id = %d", tableName, point)).Scan(&binary); err != nil {
		if point == 0 {
			// because 0 it's empty table
			return nil, nil
		}
		return nil, err
	}

	var block types.Block
	if err = block.UnmarshalSSZ(binary); err != nil {
		return nil, err
	}

	return &block, nil
}
