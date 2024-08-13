package cliservice

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func (s *Service) handleCurrencyTx(txHash common.Hash, contractAddr types.Address) error {
	s.logger.Info().
		Stringer(logging.FieldShardId, contractAddr.ShardId()).
		Stringer(logging.FieldMessageHash, txHash).
		Send()

	_, err := s.WaitForReceipt(contractAddr.ShardId(), txHash)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to wait for currency transaction receipt")
		return err
	}
	return nil
}

func (s *Service) CurrencyCreate(contractAddr types.Address, amount types.Value, name string) (*types.CurrencyId, error) {
	txHash, err := s.client.SetCurrencyName(contractAddr, name, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send setCurrencyName transaction")
		return nil, err
	}
	if err = s.handleCurrencyTx(txHash, contractAddr); err != nil {
		return nil, err
	}

	txHash, err = s.client.CurrencyMint(contractAddr, amount, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send minCurrency transaction")
		return nil, err
	}
	if err = s.handleCurrencyTx(txHash, contractAddr); err != nil {
		return nil, err
	}

	currencyId := types.CurrencyIdForAddress(contractAddr)
	s.logger.Info().Stringer(logging.FieldCurrencyId, common.BytesToHash(currencyId[:])).Msgf("Created %v:%v", name, amount)
	return currencyId, nil
}

func (s *Service) CurrencyMint(contractAddr types.Address, amount types.Value) (common.Hash, error) {
	txHash, err := s.client.CurrencyMint(contractAddr, amount, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send transfer transaction")
		return common.EmptyHash, err
	}

	if err = s.handleCurrencyTx(txHash, contractAddr); err != nil {
		return common.EmptyHash, err
	}
	currencyId := types.CurrencyIdForAddress(contractAddr)
	s.logger.Info().Stringer(logging.FieldCurrencyId, common.BytesToHash(currencyId[:])).Msgf("Minted %v", amount)
	return txHash, nil
}
