package ibft

import (
	"github.com/NilFoundation/nil/nil/common/logging"
	protoIBFT "github.com/NilFoundation/nil/nil/go-ibft/messages/proto"
	"google.golang.org/protobuf/proto"
)

func (i *backendIBFT) signMessage(msg *protoIBFT.IbftMessage) *protoIBFT.IbftMessage {
	raw, err := proto.Marshal(msg)
	if err != nil {
		i.logger.Error().Err(err).Msg("failed to marshal message")
		return nil
	}

	if msg.Signature, err = i.signer.Sign(raw); err != nil {
		event := i.logger.Error().Err(err).
			Stringer("type", msg.GetType())
		if view := msg.GetView(); view != nil {
			event = event.Uint64(logging.FieldHeight, view.Height).
				Uint64(logging.FieldRound, view.Round)
		}
		event.Msg("Failed to sign a message")
		return nil
	}
	return msg
}

func (i *backendIBFT) BuildPrePrepareMessage(
	rawProposal []byte,
	certificate *protoIBFT.RoundChangeCertificate,
	view *protoIBFT.View,
) *protoIBFT.IbftMessage {
	proposalMsg := &protoIBFT.Proposal{
		RawProposal: rawProposal,
		Round:       view.Round,
	}

	proposal, err := i.unmarshalProposal(rawProposal)
	if err != nil {
		i.logger.Error().Err(err).Msg("failed to unmarshal proposal")
		return nil
	}

	msg := &protoIBFT.IbftMessage{
		View: view,
		From: i.ID(),
		Type: protoIBFT.MessageType_PREPREPARE,
		Payload: &protoIBFT.IbftMessage_PreprepareData{
			PreprepareData: &protoIBFT.PrePrepareMessage{
				Proposal:     proposalMsg,
				ProposalHash: proposal.BlockHash.Bytes(),
				Certificate:  certificate,
			},
		},
	}

	return i.signMessage(msg)
}

func (i *backendIBFT) BuildPrepareMessage(proposalHash []byte, view *protoIBFT.View) *protoIBFT.IbftMessage {
	msg := &protoIBFT.IbftMessage{
		View: view,
		From: i.ID(),
		Type: protoIBFT.MessageType_PREPARE,
		Payload: &protoIBFT.IbftMessage_PrepareData{
			PrepareData: &protoIBFT.PrepareMessage{
				ProposalHash: proposalHash,
			},
		},
	}

	return i.signMessage(msg)
}

func (i *backendIBFT) BuildCommitMessage(proposalHash []byte, view *protoIBFT.View) *protoIBFT.IbftMessage {
	seal, err := i.signer.SignHash(proposalHash)
	if err != nil {
		i.logger.Error().Err(err).
			Hex(logging.FieldPublicKey, i.signer.GetPublicKey()).
			Hex(logging.FieldSignature, seal).
			Hex(logging.FieldBlockHash, proposalHash).
			Msg("Failed to sign a proposal hash")
		return nil
	}

	msg := &protoIBFT.IbftMessage{
		View: view,
		From: i.ID(),
		Type: protoIBFT.MessageType_COMMIT,
		Payload: &protoIBFT.IbftMessage_CommitData{
			CommitData: &protoIBFT.CommitMessage{
				ProposalHash:  proposalHash,
				CommittedSeal: seal,
			},
		},
	}

	return i.signMessage(msg)
}

func (i *backendIBFT) BuildRoundChangeMessage(
	proposal *protoIBFT.Proposal,
	certificate *protoIBFT.PreparedCertificate,
	view *protoIBFT.View,
) *protoIBFT.IbftMessage {
	msg := &protoIBFT.IbftMessage{
		View: view,
		From: i.ID(),
		Type: protoIBFT.MessageType_ROUND_CHANGE,
		Payload: &protoIBFT.IbftMessage_RoundChangeData{RoundChangeData: &protoIBFT.RoundChangeMessage{
			LastPreparedProposal:      proposal,
			LatestPreparedCertificate: certificate,
		}},
	}

	return i.signMessage(msg)
}
