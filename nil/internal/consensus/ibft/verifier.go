package ibft

import (
	"bytes"
	"slices"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/go-ibft/messages"
	protoIBFT "github.com/NilFoundation/nil/nil/go-ibft/messages/proto"
	"github.com/NilFoundation/nil/nil/internal/config"
)

func (i *backendIBFT) IsValidProposal(rawProposal []byte) bool {
	proposal, err := i.unmarshalProposal(rawProposal)
	if err != nil {
		return false
	}

	_, err = i.validator.VerifyProposal(i.ctx, proposal)
	return err == nil
}

func (i *backendIBFT) IsValidValidator(msg *protoIBFT.IbftMessage) bool {
	msgNoSig, err := msg.PayloadNoSig()
	if err != nil {
		return false
	}

	// Here we use transportCtx because this method could be called from the transport goroutine
	validators, err := i.validatorsCache.getValidators(i.transportCtx, msg.View.Height)
	if err != nil {
		i.logger.Error().
			Err(err).
			Uint64(logging.FieldRound, msg.View.Round).
			Uint64(logging.FieldHeight, msg.View.Height).
			Msg("Failed to get validators")
		return false
	}

	index := slices.IndexFunc(validators, func(v config.ValidatorInfo) bool {
		return bytes.Equal(v.PublicKey[:], msg.From)
	})
	if index == -1 {
		event := i.logger.Error().
			Hex("key", msg.From)

		if view := msg.GetView(); view != nil {
			event = event.Uint64(logging.FieldHeight, view.Height).
				Uint64(logging.FieldRound, view.Round)
		}
		event.Msg("Key not found in validators list")
		return false
	}

	validator := validators[index]
	if !i.signer.VerifyWithKey(validator.PublicKey[:], msgNoSig, msg.Signature) {
		event := i.logger.Error()
		if view := msg.GetView(); view != nil {
			event = event.Uint64(logging.FieldHeight, view.Height).
				Uint64(logging.FieldRound, view.Round)
		}
		event.Msg("Invalid signature")
		return false
	}

	return true
}

func (i *backendIBFT) IsProposer(id []byte, height, round uint64) bool {
	proposer, err := i.calcProposer(height, round)
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
	_ []byte,
	committedSeal *messages.CommittedSeal,
) bool {
	return true
}
