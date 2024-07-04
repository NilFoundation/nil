package service

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
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

func (s *Service) CurrencyCreate(contractAddr types.Address, amount types.Value, name string, withdraw bool) error {
	txHash, err := s.client.CurrencyCreate(contractAddr, amount, name, withdraw, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send create transaction")
		return err
	}

	if err = s.handleCurrencyTx(txHash, contractAddr); err != nil {
		return err
	}
	currencyId := types.CurrencyIdForAddress(contractAddr)
	s.logger.Info().Stringer(logging.FieldCurrencyId, common.BytesToHash(currencyId[:])).Msgf("Created %v:%v", name, amount)
	return nil
}

func (s *Service) CurrencyWithdraw(contractAddr types.Address, amount types.Value, toAddr types.Address) error {
	txHash, err := s.client.CurrencyWithdraw(contractAddr, amount, toAddr, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send transfer transaction")
		return err
	}

	if err = s.handleCurrencyTx(txHash, contractAddr); err != nil {
		return err
	}
	currencyId := types.CurrencyIdForAddress(contractAddr)
	s.logger.Info().Stringer(logging.FieldCurrencyId, common.BytesToHash(currencyId[:])).Msgf("Transferred %v to %v", amount, toAddr)
	return nil
}

func (s *Service) CurrencyMint(contractAddr types.Address, amount types.Value, withdraw bool) error {
	txHash, err := s.client.CurrencyMint(contractAddr, amount, withdraw, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send transfer transaction")
		return err
	}

	if err = s.handleCurrencyTx(txHash, contractAddr); err != nil {
		return err
	}
	currencyId := types.CurrencyIdForAddress(contractAddr)
	s.logger.Info().Stringer(logging.FieldCurrencyId, common.BytesToHash(currencyId[:])).Msgf("Minted %v", amount)
	return nil
}
