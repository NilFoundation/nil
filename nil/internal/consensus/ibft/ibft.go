package ibft

import (
	"context"
	"crypto/ecdsa"
	"math/big"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/go-ibft/core"
	"github.com/NilFoundation/nil/nil/go-ibft/messages"
	protoIBFT "github.com/NilFoundation/nil/nil/go-ibft/messages/proto"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
)

const ibftProto = "/ibft/0.2"

type ConsensusParams struct {
	ShardId    types.ShardId
	Db         db.DB
	Validator  validator
	NetManager *network.Manager
	PrivateKey *ecdsa.PrivateKey
	Validators []config.ValidatorInfo
}

type validator interface {
	BuildProposal(ctx context.Context) (*execution.ProposalSSZ, error)
	VerifyProposal(ctx context.Context, proposal *execution.ProposalSSZ) (*types.Block, error)
	InsertProposal(ctx context.Context, proposal *execution.ProposalSSZ, sig types.Signature) error
}

type backendIBFT struct {
	ctx        context.Context
	db         db.DB
	consensus  *core.IBFT
	shardId    types.ShardId
	validator  validator
	logger     zerolog.Logger
	nm         *network.Manager
	transport  transport
	signer     *Signer
	validators []config.ValidatorInfo
}

var _ core.Backend = &backendIBFT{}

func (i *backendIBFT) unmarshalProposal(raw []byte) (*execution.ProposalSSZ, error) {
	proposal := &execution.ProposalSSZ{}
	if err := proposal.UnmarshalSSZ(raw); err != nil {
		return nil, err
	}
	return proposal, nil
}

func (i *backendIBFT) BuildProposal(view *protoIBFT.View) []byte {
	proposal, err := i.validator.BuildProposal(i.ctx)
	if err != nil {
		i.logger.Error().Err(err).Msg("failed to build proposal")
		return nil
	}
	data, err := proposal.MarshalSSZ()
	if err != nil {
		i.logger.Error().Err(err).Msg("failed to marshal proposal")
		return nil
	}
	return data
}

func (i *backendIBFT) InsertProposal(proposal *protoIBFT.Proposal, committedSeals []*messages.CommittedSeal) {
	proposalBlock, err := i.unmarshalProposal(proposal.RawProposal)
	if err != nil {
		i.logger.Error().Err(err).Msg("failed to unmarshal proposal")
		return
	}

	var signature types.Signature
	for _, seal := range committedSeals {
		if len(seal.Signature) != 0 {
			signature = seal.Signature
		}
	}

	if err := i.validator.InsertProposal(i.ctx, proposalBlock, signature); err != nil {
		i.logger.Error().Err(err).Msg("fail to insert proposal")
	}
}

func (i *backendIBFT) ID() []byte {
	return i.signer.GetPublicKey()
}

func (i *backendIBFT) isActiveValidator() bool {
	return true
}

func NewConsensus(cfg *ConsensusParams) *backendIBFT {
	logger := logging.NewLogger("consensus").With().Stringer(logging.FieldShardId, cfg.ShardId).Logger()
	l := &ibftLogger{
		logger: logger.With().CallerWithSkipFrameCount(3).Logger(),
	}

	backend := &backendIBFT{
		db:         cfg.Db,
		shardId:    cfg.ShardId,
		validator:  cfg.Validator,
		logger:     logger,
		nm:         cfg.NetManager,
		signer:     NewSigner(cfg.PrivateKey),
		validators: cfg.Validators,
	}
	backend.consensus = core.NewIBFT(l, backend, backend)
	return backend
}

func (i *backendIBFT) GetVotingPowers(height uint64) (map[string]*big.Int, error) {
	result := make(map[string]*big.Int, len(i.validators))
	for _, v := range i.validators {
		result[string(v.PublicKey[:])] = big.NewInt(1)
	}
	return result, nil
}

func (i *backendIBFT) Init(ctx context.Context) error {
	if i.nm == nil {
		i.setupLocalTransport()
		return nil
	}
	return i.setupTransport(ctx)
}

func (i *backendIBFT) RunSequence(ctx context.Context, height uint64) error {
	i.ctx = ctx
	i.consensus.RunSequence(ctx, height)
	return nil
}
