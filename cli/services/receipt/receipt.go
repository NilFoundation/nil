package receipt

import (
	"encoding/json"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
)

var logger = logging.NewLogger("receiptService")

type Service struct {
	client  client.Client
	shardId types.ShardId
}

// NewService initializes a new Service with the given client
func NewService(c client.Client, shardId types.ShardId) *Service {
	return &Service{
		client:  c,
		shardId: shardId,
	}
}

// FetchReceiptByHash fetches the receipt by hash
func (s *Service) FetchReceiptByHash(receiptHash string) ([]byte, error) {
	hash := common.BytesToHash([]byte(receiptHash))
	receiptData, err := s.client.GetInMessageReceipt(s.shardId, hash)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch receipt")
		return nil, err
	}

	receiptDataJSON, err := json.MarshalIndent(receiptData, "", "  ")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal receipt data to JSON")
		return nil, err
	}

	logger.Trace().Msgf("Fetched receipt:\n%s", receiptDataJSON)

	return receiptDataJSON, nil
}
