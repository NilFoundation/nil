package ibft

import (
	"bytes"
	"errors"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/go-ibft/messages"
	protoIBFT "github.com/NilFoundation/nil/nil/go-ibft/messages/proto"
	cerrors "github.com/NilFoundation/nil/nil/internal/collate/errors"
	"github.com/NilFoundation/nil/nil/internal/config"
)

func (i *backendIBFT) IsValidProposal(rawProposal []byte) bool {
	proposal, err := i.unmarshalProposal(rawProposal)
	if err != nil {
		i.logger.Error().
			Err(err).
			Uint64(logging.FieldHeight, uint64(proposal.PrevBlockId)+1).
			Msg("Failed to unmarshal proposal")
		return false
	}

	_, err = i.validator.VerifyProposal(i.ctx, proposal)
	if err != nil {
		event := i.logger.Error()
		if errors.Is(err, cerrors.ErrOldBlock) {
			event = i.logger.Debug()
		}
		event.Err(err).
			Uint64(logging.FieldHeight, uint64(proposal.PrevBlockId)+1).
			Msg("Proposal is invalid")
	}
	return err == nil
}

func (i *backendIBFT) IsValidValidator(msg *protoIBFT.IbftMessage) bool {
	msgNoSig, err := msg.PayloadNoSig()
	if err != nil {
		return false
	}

	// Here (and below) we use transportCtx because this method could be called from the transport goroutine
	// or i.ctx can be changed in case we start new sequence for the next height.
	lastBlock, _, err := i.validator.GetLastBlock(i.transportCtx)
	if err != nil {
		i.logger.Error().
			Err(err).
			Msg("Failed to get last block")
		return false
	}

	var height uint64
	loggerCtx := i.logger.With().Hex(logging.FieldPublicKey, msg.From)
	if view := msg.GetView(); view != nil {
		loggerCtx = loggerCtx.
			Uint64(logging.FieldHeight, view.Height).
			Uint64(logging.FieldRound, view.Round)
		height = view.Height
	}
	logger := loggerCtx.Logger()

	// Current message is from future.
	// Some validator could commit block and start new sequence before we committed that block.
	// Use last known config since validators list is static for now.
	// TODO: consider some options to fix it.
	if expectedHeight := uint64(lastBlock.Id + 1); height > expectedHeight {
		logger.Warn().Msgf("Got message with height=%d while expected=%d", height, expectedHeight)
		height = expectedHeight
	}

	params, err := config.GetConfigParams(i.transportCtx, i.txFabric, i.shardId, height)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("Failed to get validators")
		return false
	}

	_, ok := params.PublicKeys.Find(config.Pubkey(msg.From))
	if !ok {
		logger.Error().
			Msg("public key not found in validators list")
		return false
	}

	if err := i.signer.VerifyWithKey(msg.From, msgNoSig, msg.Signature); err != nil {
		logger.Err(err).Msg("Failed to verify signature")
		return false
	}

	return true
}

func (i *backendIBFT) getPrevProposer(height uint64) *uint64 {
	// It doesn't make sense for 0 block
	// For the first block we should start from the first validator (offset = 0)
	if height < 2 {
		return nil
	}

	block, _, err := i.validator.GetLastBlock(i.ctx)
	if err != nil {
		return nil
	}

	return &block.ProposerIndex
}

func (i *backendIBFT) IsProposer(id []byte, height, round uint64) bool {
	prevProposerIndex := i.getPrevProposer(height)
	proposer, _, err := i.calcProposer(height, round, prevProposerIndex)
	if err != nil {
		i.logger.Error().
			Err(err).
			Uint64(logging.FieldHeight, height).
			Uint64(logging.FieldRound, round).
			Msg("Failed to calculate proposer")
		return false
	}
	return bytes.Equal(proposer.PublicKey[:], id)
}

func (i *backendIBFT) IsValidProposalHash(proposal *protoIBFT.Proposal, hash []byte) bool {
	prop, err := i.unmarshalProposal(proposal.RawProposal)
	if err != nil {
		return false
	}

	block, err := i.validator.VerifyProposal(i.ctx, prop)
	if err != nil {
		event := i.logger.Error()
		if errors.Is(err, cerrors.ErrOldBlock) {
			event = i.logger.Debug()
		}

		event.Err(err).
			Uint64(logging.FieldRound, proposal.Round).
			Uint64(logging.FieldHeight, uint64(prop.PrevBlockId)+1).
			Msg("Failed to verify proposal")
		return false
	}

	blockHash := block.Hash(i.shardId)
	isValid := bytes.Equal(blockHash.Bytes(), hash)
	if !isValid {
		i.logger.Error().
			Stringer("expected", blockHash).
			Hex("got", hash).
			Uint64(logging.FieldRound, proposal.Round).
			Uint64(logging.FieldHeight, uint64(prop.PrevBlockId)+1).
			Msg("Invalid proposal hash")
	}
	return isValid
}

func (i *backendIBFT) IsValidCommittedSeal(
	proposalHash []byte,
	committedSeal *messages.CommittedSeal,
) bool {
	if err := i.signer.VerifyWithKeyHash(committedSeal.Signer, proposalHash, committedSeal.Signature); err != nil {
		i.logger.Error().
			Err(err).
			Hex(logging.FieldPublicKey, committedSeal.Signer).
			Hex(logging.FieldSignature, committedSeal.Signature).
			Hex(logging.FieldBlockHash, proposalHash).
			Msg("Failed to verify signature")
		return false
	}
	return true
}
