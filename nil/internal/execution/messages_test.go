package execution

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/stretchr/testify/suite"
)

type MessagesSuite struct {
	suite.Suite

	ctx context.Context
	db  db.DB
}

func (s *MessagesSuite) SetupSuite() {
	s.ctx = context.Background()
}

func (s *MessagesSuite) SetupTest() {
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
}

func (s *MessagesSuite) TearDownTest() {
	s.db.Close()
}

func (s *MessagesSuite) TestValidateExternalMessage() {
	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	es, err := NewExecutionStateForShard(tx, types.BaseShardId, common.NewTestTimer(0), 1)
	s.Require().NoError(err)
	s.Require().NotNil(es)

	validate := func(msg *types.Message) types.ExecError {
		res := ValidateExternalMessage(es, msg)
		s.Require().False(res.IsFatal())
		if res.Failed() {
			return res.Error
		}
		return nil
	}

	s.Run("Deploy", func() {
		code := types.Code("some-code")
		msg := &types.Message{
			Flags: types.NewMessageFlags(types.MessageFlagDeploy),
			To:    types.GenerateRandomAddress(types.BaseShardId),
			Data:  types.BuildDeployPayload(code, common.EmptyHash).Bytes(),
		}

		s.Run("NoAccount", func() {
			s.Require().Equal(types.ErrorNoAccountToPayFees, validate(msg).Code())

			s.Require().NoError(es.CreateAccount(msg.To))
		})

		s.Run("IncorrectAddress", func() {
			s.Require().Equal(types.ErrorIncorrectDeploymentAddress, validate(msg).Code())

			msg.To = types.CreateAddress(types.BaseShardId, *types.ParseDeployPayload(msg.Data))
			s.Require().NoError(es.CreateAccount(msg.To))
		})

		s.Run("Ok", func() {
			s.Require().NoError(validate(msg))
		})

		s.Run("ContractAlreadyExists", func() {
			s.Require().NoError(es.SetCode(msg.To, code))

			s.Require().Equal(types.ErrorContractAlreadyExists, validate(msg).Code())
		})
	})

	s.Run("Execution", func() {
		msg := &types.Message{
			To:   types.GenerateRandomAddress(types.BaseShardId),
			Data: []byte("hello"),
		}

		s.Run("NoAccount", func() {
			s.Require().Equal(types.ErrorNoAccountToPayFees, validate(msg).Code())

			s.Require().NoError(es.CreateAccount(msg.To))
		})

		s.Run("NoContract", func() {
			s.Require().Equal(types.ErrorContractDoesNotExist, validate(msg).Code())

			// contract that always returns "true",
			// so verifies any message
			s.Require().NoError(es.SetCode(msg.To, hexutil.FromHex("600160005260206000f3")))
		})

		s.Run("NoBalance", func() {
			s.Require().ErrorIs(validate(msg), vm.ErrOutOfGas)

			s.Require().NoError(es.SetBalance(msg.To, types.NewValueFromUint64(10_000_000)))
		})

		s.Run("Ok", func() {
			s.Require().NoError(validate(msg))
		})

		// todo: fail signature verification

		s.Run("InvalidChain", func() {
			msg.ChainId = 100500
			s.Require().Equal(types.ErrorInvalidChainId, validate(msg).Code())
		})

		s.Run("SeqnoGap", func() {
			msg.ChainId = types.DefaultChainId
			msg.Seqno = 100
			s.Require().Equal(types.ErrorSeqnoGap, validate(msg).Code())
		})
	})
}

func (s *MessagesSuite) TestValidateDeployMessage() {
	msg := &types.Message{
		Data: types.Code("no-salt"),
	}

	// data too short
	s.Require().Equal(types.ErrorInvalidPayload, types.GetErrorCode(ValidateDeployMessage(msg)))

	// Deploy to the main shard from base shard - FAIL
	data := types.BuildDeployPayload([]byte("some-code"), common.EmptyHash)
	msg.To = types.CreateAddress(types.MainShardId, data)
	msg.Data = data.Bytes()
	s.Require().Equal(types.ErrorDeployToMainShard, types.GetErrorCode(ValidateDeployMessage(msg)))

	// Deploy to base shard
	msg.To = types.CreateAddress(types.BaseShardId, data)
	s.Require().NoError(ValidateDeployMessage(msg))
}

func TestMessages(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MessagesSuite))
}
