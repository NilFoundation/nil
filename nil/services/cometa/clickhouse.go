package cometa

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type StorageClick struct {
	conn       driver.Conn
	insertConn driver.Conn
}

const SchemaVersion = 1

var _ Storage = new(StorageClick)

func NewStorageClick(ctx context.Context, cfg *Config) (*StorageClick, error) {
	connectionOptions := clickhouse.Options{
		Auth: clickhouse.Auth{
			Database: cfg.DbName,
			Username: cfg.DbUser,
			Password: cfg.DbPassword,
		},
		Addr: []string{cfg.DbEndpoint},
	}
	conn, err := clickhouse.Open(&connectionOptions)
	if err != nil {
		return nil, err
	}

	insertConn, err := clickhouse.Open(&connectionOptions)
	if err != nil {
		return nil, err
	}

	err = conn.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS contracts_metadata
			(address FixedString(20), version UInt32, data_json String, code_hash FixedString(32), abi String, source_code Map(String, String))
			ENGINE = MergeTree
			PRIMARY KEY (address, code_hash)
			ORDER BY (address, code_hash)`)
	if err != nil {
		return nil, fmt.Errorf("failed to create contracts_metadata table: %w", err)
	}

	err = conn.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS abi_metadata
			(address FixedString(20), selector FixedString(4), name String, type String)
			ENGINE = ReplacingMergeTree
			PRIMARY KEY (address, selector)
			ORDER BY (address, selector)`)
	if err != nil {
		return nil, fmt.Errorf("failed to create abi_metadata table: %w", err)
	}

	return &StorageClick{
		conn:       conn,
		insertConn: insertConn,
	}, nil
}

func (s *StorageClick) StoreContract(ctx context.Context, contractData *ContractData, address types.Address) error {
	data, err := json.Marshal(contractData)
	if err != nil {
		return fmt.Errorf("failed to marshal contract data: %w", err)
	}

	err = s.insertConn.Exec(ctx, `INSERT INTO contracts_metadata
    	(address, data_json, code_hash, abi, source_code, version)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		string(address.Bytes()), string(data), string(types.Code(contractData.Code).Hash().Bytes()), contractData.Abi,
		contractData.SourceCode, SchemaVersion)
	if err != nil {
		return fmt.Errorf("failed to insert contract data: %w", err)
	}

	var abiSpec abi.ABI
	abiSpec, err = abi.JSON(strings.NewReader(contractData.Abi))
	if err != nil {
		return fmt.Errorf("failed to parse abi: %w", err)
	}
	for _, method := range abiSpec.Methods {
		err = s.insertConn.Exec(ctx, `INSERT INTO abi_metadata
			(address, selector, name, type)
			VALUES ($1, $2, $3, $4)`,
			string(address.Bytes()), string(method.ID), method.Name, "method")
		if err != nil {
			return fmt.Errorf("failed to insert abi metadata: %w", err)
		}
	}

	return nil
}

func (s *StorageClick) LoadContractData(ctx context.Context, address types.Address) (*ContractData, error) {
	row := s.conn.QueryRow(ctx, `SELECT data_json FROM contracts_metadata WHERE address = $1`, string(address.Bytes()))

	var str string
	if err := row.Scan(&str); err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	res := new(ContractData)
	if err := json.Unmarshal([]byte(str), res); err != nil {
		return nil, err
	}

	return res, nil
}

func (s *StorageClick) GetAbi(ctx context.Context, address types.Address) (string, error) {
	row := s.conn.QueryRow(ctx, `SELECT abi FROM contracts_metadata WHERE address = $1`, string(address.Bytes()))

	var str string
	if err := row.Scan(&str); err != nil {
		return "", fmt.Errorf("failed to scan row: %w", err)
	}
	return str, nil
}

func (s *StorageClick) LoadContractDataByCodeHash(ctx context.Context, codeHash common.Hash) (*ContractData, error) {
	row := s.conn.QueryRow(ctx, `SELECT data_json FROM contracts_metadata WHERE code_hash = $1`,
		string(codeHash.Bytes()))

	var str string
	if err := row.Scan(&str); err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	res := new(ContractData)
	if err := json.Unmarshal([]byte(str), res); err != nil {
		return nil, err
	}

	return res, nil
}
