package core

import (
	"bytes"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/go-ibft/messages"
	"github.com/NilFoundation/nil/nil/go-ibft/messages/proto"
	"github.com/stretchr/testify/assert"
)

func TestMain(t *testing.T) {
	t.Parallel()
	var multicastFn func(message *proto.Message)

	proposal := []byte("proposal")
	proposalHash := []byte("proposal hash")
	committedSeal := []byte("seal")
	numNodes := uint64(4)
	nodes := generateNodeAddresses(numNodes)
	insertedBlocks := make([][]byte, numNodes)

	ilogger := logging.NewLogger("IBFT")

	commonLoggerCallback := func(logger *mockLogger) {
		logger.infoFn = func(s string, i ...interface{}) {
			ilogger.Info().Msgf(s, i...)
		}
		logger.debugFn = func(s string, i ...interface{}) {
			ilogger.Debug().Msgf(s, i...)
		}
		logger.errorFn = func(s string, i ...interface{}) {
			ilogger.Error().Msgf(s, i...)
		}
	}

	// commonTransportCallback is the common method modification
	// required for Transport, for all nodes
	commonTransportCallback := func(transport *mockTransport) {
		transport.multicastFn = func(message *proto.Message) {
			multicastFn(message)
		}
	}

	// commonBackendCallback is the common method modification required
	// for the Backend, for all nodes
	commonBackendCallback := func(backend *mockBackend, nodeIndex int) {
		// Make sure the quorum function requires all nodes
		backend.quorumFn = func(_ uint64) uint64 {
			return numNodes
		}

		// Make sure the node ID is properly relayed
		backend.idFn = func() []byte {
			return nodes[nodeIndex]
		}

		// Make sure the only proposer is node 0
		backend.isProposerFn = func(from []byte, _ uint64, _ uint64) bool {
			return bytes.Equal(from, nodes[0])
		}

		// Make sure the proposal is valid if it matches what node 0 proposed
		backend.isValidBlockFn = func(newProposal []byte) bool {
			return bytes.Equal(newProposal, proposal)
		}

		// Make sure the proposal hash matches
		backend.isValidProposalHashFn = func(p []byte, ph []byte) bool {
			return bytes.Equal(p, proposal) && bytes.Equal(ph, proposalHash)
		}

		// Make sure the preprepare message is built correctly
		backend.buildPrePrepareMessageFn = func(
			proposal []byte,
			certificate *proto.RoundChangeCertificate,
			view *proto.View,
		) *proto.Message {
			return buildBasicPreprepareMessage(
				proposal,
				proposalHash,
				certificate,
				nodes[nodeIndex],
				view)
		}

		// Make sure the prepare message is built correctly
		backend.buildPrepareMessageFn = func(_ []byte, view *proto.View) *proto.Message {
			return buildBasicPrepareMessage(proposalHash, nodes[nodeIndex], view)
		}

		// Make sure the commit message is built correctly
		backend.buildCommitMessageFn = func(_ []byte, view *proto.View) *proto.Message {
			return buildBasicCommitMessage(proposalHash, committedSeal, nodes[nodeIndex], view)
		}

		// Make sure the round change message is built correctly
		backend.buildRoundChangeMessageFn = func(
			proposal []byte,
			certificate *proto.PreparedCertificate,
			view *proto.View,
		) *proto.Message {
			return buildBasicRoundChangeMessage(proposal, certificate, view, nodes[nodeIndex])
		}

		// Make sure the inserted proposal is noted
		backend.insertBlockFn = func(proposal []byte, _ []*messages.CommittedSeal) {
			insertedBlocks[nodeIndex] = proposal
		}
	}

	var (
		backendCallbackMap = map[int]backendConfigCallback{
			0: func(backend *mockBackend) {
				// Execute the common backend setup
				commonBackendCallback(backend, 0)

				// Set the proposal creation method for node 0, since
				// they are the proposer
				backend.buildProposalFn = func(_ uint64) []byte {
					return proposal
				}
			},
			1: func(backend *mockBackend) {
				commonBackendCallback(backend, 1)
			},
			2: func(backend *mockBackend) {
				commonBackendCallback(backend, 2)
			},
			3: func(backend *mockBackend) {
				commonBackendCallback(backend, 3)
			},
		}
		transportCallbackMap = map[int]transportConfigCallback{
			0: commonTransportCallback,
			1: commonTransportCallback,
			2: commonTransportCallback,
			3: commonTransportCallback,
		}
		loggerCallbackMap = map[int]loggerConfigCallback{
			0: commonLoggerCallback,
			1: commonLoggerCallback,
			2: commonLoggerCallback,
			3: commonLoggerCallback,
		}
	)

	// Create the mock cluster
	cluster := newMockCluster(
		numNodes,
		backendCallbackMap,
		loggerCallbackMap,
		transportCallbackMap,
	)

	// Set the multicast callback to relay the message
	// to the entire cluster
	multicastFn = func(message *proto.Message) {
		cluster.pushMessage(message)
	}

	// Start the main run loops
	cluster.runSequence(0)

	// Wait until the main run loops finish
	cluster.awaitCompletion()

	// Make sure the inserted blocks match what node 0 proposed
	for _, block := range insertedBlocks {
		assert.True(t, bytes.Equal(block, proposal))
	}
}
