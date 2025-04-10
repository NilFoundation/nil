package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/indexer/driver"
	indexertypes "github.com/NilFoundation/nil/nil/services/indexer/types"
	"github.com/stretchr/testify/suite"
)

type SuiteServiceTest struct {
	suite.Suite

	service *Service
	client  client.ClientMock
	ctx     context.Context
	cancel  context.CancelFunc
	dbPath  string
}

func (s *SuiteServiceTest) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.client = client.ClientMock{}

	// Create a temporary directory for the test database
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	s.Require().NoError(err)
	s.dbPath = filepath.Join(tmpDir, "test.db")

	// Create service with BadgerDB
	cfg := &Config{
		UseBadger: true,
		DbPath:    s.dbPath,
	}
	service, err := NewService(s.ctx, cfg)
	s.Require().NoError(err)
	s.service = service
}

func (s *SuiteServiceTest) TearDownTest() {
	// Clean up the temporary directory
	err := os.RemoveAll(filepath.Dir(s.dbPath))
	s.Require().NoError(err)
}

func (s *SuiteServiceTest) TestGetAddressActions() {
	// Create test addresses
	addr1 := types.HexToAddress("0x1234567890123456789012345678901234567890")
	addr2 := types.HexToAddress("0x1234567890123456789012345678901234567891")

	// Create test transactions
	tx1 := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			To: addr2,
		},
		From:  addr1,
		Value: types.NewValueFromUint64(100),
	}
	tx1Hash := tx1.Hash()

	tx2 := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			To: addr1,
		},
		From:  addr2,
		Value: types.NewValueFromUint64(200),
	}
	tx2Hash := tx2.Hash()

	// Create test receipts
	receipt1 := &types.Receipt{
		Success: true,
		TxnHash: tx1Hash,
	}
	receipt2 := &types.Receipt{
		Success: true,
		TxnHash: tx2Hash,
	}

	// Create test blocks with transactions
	blocks := []*driver.BlockWithShardId{
		{
			BlockWithExtractedData: &types.BlockWithExtractedData{
				Block: &types.Block{
					BlockData: types.BlockData{
						Id: 1,
					},
				},
				InTransactions: []*types.Transaction{tx1},
				Receipts:       []*types.Receipt{receipt1},
			},
			ShardId: 0,
		},
		{
			BlockWithExtractedData: &types.BlockWithExtractedData{
				Block: &types.Block{
					BlockData: types.BlockData{
						Id: 2,
					},
				},
				InTransactions: []*types.Transaction{tx2},
				Receipts:       []*types.Receipt{receipt2},
			},
			ShardId: 0,
		},
	}

	// Index the blocks
	err := s.service.Driver.IndexBlocks(s.ctx, blocks)
	s.Require().NoError(err)

	// Test cases
	tests := []struct {
		name     string
		address  types.Address
		since    types.BlockNumber
		expected []indexertypes.AddressAction
	}{
		{
			name:    "Get all actions for addr1",
			address: addr1,
			since:   0,
			expected: []indexertypes.AddressAction{
				{
					Hash:    tx1Hash,
					From:    addr1,
					To:      addr2,
					Amount:  types.NewValueFromUint64(100),
					BlockId: 1,
					Type:    indexertypes.SendEth,
					Status:  indexertypes.Success,
				},
				{
					Hash:    tx2Hash,
					From:    addr2,
					To:      addr1,
					Amount:  types.NewValueFromUint64(200),
					BlockId: 2,
					Type:    indexertypes.ReceiveEth,
					Status:  indexertypes.Success,
				},
			},
		},
		{
			name:    "Get actions for addr1 since timestamp 1500",
			address: addr1,
			since:   2,
			expected: []indexertypes.AddressAction{
				{
					Hash:    tx2Hash,
					From:    addr2,
					To:      addr1,
					Amount:  types.NewValueFromUint64(200),
					BlockId: 2,
					Type:    indexertypes.ReceiveEth,
					Status:  indexertypes.Success,
				},
			},
		},
		{
			name:     "Get actions for non-existent address",
			address:  types.HexToAddress("0x1234567890123456789012345678901234567893"),
			since:    0,
			expected: []indexertypes.AddressAction{},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			actions, err := s.service.GetAddressActions(context.Background(), tt.address, tt.since)
			s.Require().NoError(err)
			s.Equal(tt.expected, actions)
		})
	}
}

func TestServiceSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteServiceTest))
}
