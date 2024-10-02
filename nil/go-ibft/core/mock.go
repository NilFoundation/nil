package core

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/nil/go-ibft/messages"
	"github.com/NilFoundation/nil/nil/go-ibft/messages/proto"
)

// Define delegation methods
type (
	isValidBlockDelegate         func([]byte) bool
	isValidSenderDelegate        func(*proto.Message) bool
	isProposerDelegate           func([]byte, uint64, uint64) bool
	buildProposalDelegate        func(uint64) []byte
	isValidProposalHashDelegate  func([]byte, []byte) bool
	isValidCommittedSealDelegate func([]byte, *messages.CommittedSeal) bool
)

type buildPrePrepareMessageDelegate func(
	[]byte,
	*proto.RoundChangeCertificate,
	*proto.View,
) *proto.Message

type (
	buildPrepareMessageDelegate     func([]byte, *proto.View) *proto.Message
	buildCommitMessageDelegate      func([]byte, *proto.View) *proto.Message
	buildRoundChangeMessageDelegate func(
		[]byte,
		*proto.PreparedCertificate,
		*proto.View,
	) *proto.Message
)

type (
	quorumDelegate             func(blockHeight uint64) uint64
	insertBlockDelegate        func([]byte, []*messages.CommittedSeal)
	idDelegate                 func() []byte
	maximumFaultyNodesDelegate func() uint64
)

// mockBackend is the mock backend structure that is configurable
type mockBackend struct {
	isValidBlockFn         isValidBlockDelegate
	isValidSenderFn        isValidSenderDelegate
	isProposerFn           isProposerDelegate
	buildProposalFn        buildProposalDelegate
	isValidProposalHashFn  isValidProposalHashDelegate
	isValidCommittedSealFn isValidCommittedSealDelegate

	buildPrePrepareMessageFn  buildPrePrepareMessageDelegate
	buildPrepareMessageFn     buildPrepareMessageDelegate
	buildCommitMessageFn      buildCommitMessageDelegate
	buildRoundChangeMessageFn buildRoundChangeMessageDelegate
	quorumFn                  quorumDelegate
	insertBlockFn             insertBlockDelegate
	idFn                      idDelegate
	maximumFaultyNodesFn      maximumFaultyNodesDelegate
}

func (m mockBackend) ID() []byte {
	if m.idFn != nil {
		return m.idFn()
	}

	return nil
}

func (m mockBackend) InsertBlock(proposal []byte, committedSeals []*messages.CommittedSeal) {
	if m.insertBlockFn != nil {
		m.insertBlockFn(proposal, committedSeals)
	}
}

func (m mockBackend) Quorum(blockNumber uint64) uint64 {
	if m.quorumFn != nil {
		return m.quorumFn(blockNumber)
	}

	return 0
}

func (m mockBackend) IsValidBlock(block []byte) bool {
	if m.isValidBlockFn != nil {
		return m.isValidBlockFn(block)
	}

	return true
}

func (m mockBackend) IsValidSender(msg *proto.Message) bool {
	if m.isValidSenderFn != nil {
		return m.isValidSenderFn(msg)
	}

	return true
}

func (m mockBackend) IsProposer(id []byte, sequence, round uint64) bool {
	if m.isProposerFn != nil {
		return m.isProposerFn(id, sequence, round)
	}

	return false
}

func (m mockBackend) BuildProposal(blockNumber uint64) []byte {
	if m.buildProposalFn != nil {
		return m.buildProposalFn(blockNumber)
	}

	return nil
}

func (m mockBackend) IsValidProposalHash(proposal, hash []byte) bool {
	if m.isValidProposalHashFn != nil {
		return m.isValidProposalHashFn(proposal, hash)
	}

	return true
}

func (m mockBackend) IsValidCommittedSeal(proposal []byte, committedSeal *messages.CommittedSeal) bool {
	if m.isValidCommittedSealFn != nil {
		return m.isValidCommittedSealFn(proposal, committedSeal)
	}

	return true
}

func (m mockBackend) MaximumFaultyNodes() uint64 {
	if m.maximumFaultyNodesFn != nil {
		return m.maximumFaultyNodesFn()
	}

	return 0
}

func (m mockBackend) BuildPrePrepareMessage(
	proposal []byte,
	certificate *proto.RoundChangeCertificate,
	view *proto.View,
) *proto.Message {
	if m.buildPrePrepareMessageFn != nil {
		return m.buildPrePrepareMessageFn(proposal, certificate, view)
	}

	return nil
}

func (m mockBackend) BuildPrepareMessage(proposal []byte, view *proto.View) *proto.Message {
	if m.buildPrepareMessageFn != nil {
		return m.buildPrepareMessageFn(proposal, view)
	}

	return nil
}

func (m mockBackend) BuildCommitMessage(proposalHash []byte, view *proto.View) *proto.Message {
	if m.buildCommitMessageFn != nil {
		return m.buildCommitMessageFn(proposalHash, view)
	}

	return nil
}

func (m mockBackend) BuildRoundChangeMessage(
	proposal []byte,
	certificate *proto.PreparedCertificate,
	view *proto.View,
) *proto.Message {
	if m.buildRoundChangeMessageFn != nil {
		return m.buildRoundChangeMessageFn(proposal, certificate, view)
	}

	return &proto.Message{
		View: &proto.View{
			Height: view.Height,
			Round:  view.Round,
		},
		Type:    proto.MessageType_ROUND_CHANGE,
		Payload: nil,
	}
}

// Define delegation methods
type multicastFnDelegate func(*proto.Message)

// mockTransport is the mock transport structure that is configurable
type mockTransport struct {
	multicastFn multicastFnDelegate
}

func (t mockTransport) Multicast(msg *proto.Message) {
	if t.multicastFn != nil {
		t.multicastFn(msg)
	}
}

// Define delegation methods
type opLogDelegate func(string, ...interface{})

// mockLogger is the mock logging structure that is configurable
type mockLogger struct {
	infoFn,
	debugFn,
	errorFn opLogDelegate
}

func (l mockLogger) Info(msg string, args ...interface{}) {
	if l.infoFn != nil {
		l.infoFn(msg, args)
	}
}

func (l mockLogger) Debug(msg string, args ...interface{}) {
	if l.debugFn != nil {
		l.debugFn(msg, args)
	}
}

func (l mockLogger) Error(msg string, args ...interface{}) {
	if l.errorFn != nil {
		l.errorFn(msg, args)
	}
}

type mockMessages struct {
	addMessageFn    func(message *proto.Message)
	pruneByHeightFn func(height uint64)

	getValidMessagesFn func(
		view *proto.View,
		messageType proto.MessageType,
		isValid func(message *proto.Message) bool,
	) []*proto.Message
	getMostRoundChangeMessagesFn func(uint64, uint64) []*proto.Message

	subscribeFn   func(details messages.SubscriptionDetails) *messages.Subscription
	unsubscribeFn func(id messages.SubscriptionID)
}

func (m mockMessages) GetValidMessages(
	view *proto.View,
	messageType proto.MessageType,
	isValid func(*proto.Message) bool,
) []*proto.Message {
	if m.getValidMessagesFn != nil {
		return m.getValidMessagesFn(view, messageType, isValid)
	}

	return nil
}

func (m mockMessages) Subscribe(details messages.SubscriptionDetails) *messages.Subscription {
	if m.subscribeFn != nil {
		return m.subscribeFn(details)
	}

	return nil
}

func (m mockMessages) Unsubscribe(id messages.SubscriptionID) {
	if m.unsubscribeFn != nil {
		m.unsubscribeFn(id)
	}
}

func (m mockMessages) AddMessage(msg *proto.Message) {
	if m.addMessageFn != nil {
		m.addMessageFn(msg)
	}
}

func (m mockMessages) PruneByHeight(height uint64) {
	if m.pruneByHeightFn != nil {
		m.pruneByHeightFn(height)
	}
}

func (m mockMessages) GetMostRoundChangeMessages(round, height uint64) []*proto.Message {
	if m.getMostRoundChangeMessagesFn != nil {
		return m.getMostRoundChangeMessagesFn(round, height)
	}

	return nil
}

type (
	backendConfigCallback   func(*mockBackend)
	loggerConfigCallback    func(*mockLogger)
	transportConfigCallback func(*mockTransport)
)

// newMockCluster creates a new IBFT cluster
func newMockCluster(
	numNodes uint64,
	backendCallbackMap map[int]backendConfigCallback,
	loggerCallbackMap map[int]loggerConfigCallback,
	transportCallbackMap map[int]transportConfigCallback,
) *mockCluster {
	if numNodes < 1 {
		return nil
	}

	nodes := make([]*IBFT, numNodes)
	nodeCtxs := make([]mockNodeContext, numNodes)

	for index := range int(numNodes) {
		var (
			logger    = &mockLogger{}
			transport = &mockTransport{}
			backend   = &mockBackend{}
		)

		// Execute set callbacks, if any
		if backendCallbackMap != nil {
			if backendCallback, isSet := backendCallbackMap[index]; isSet {
				backendCallback(backend)
			}
		}

		if loggerCallbackMap != nil {
			if loggerCallback, isSet := loggerCallbackMap[index]; isSet {
				loggerCallback(logger)
			}
		}

		if transportCallbackMap != nil {
			if transportCallback, isSet := transportCallbackMap[index]; isSet {
				transportCallback(transport)
			}
		}

		// Create a new instance of the IBFT node
		nodes[index] = NewIBFT(logger, backend, transport)

		// Instantiate context for the nodes
		ctx, cancelFn := context.WithCancel(context.Background())
		nodeCtxs[index] = mockNodeContext{
			ctx:      ctx,
			cancelFn: cancelFn,
		}
	}

	return &mockCluster{
		nodes: nodes,
		ctxs:  nodeCtxs,
	}
}

// mockNodeContext keeps track of the node runtime context
type mockNodeContext struct {
	ctx      context.Context
	cancelFn context.CancelFunc
}

// mockNodeWg is the WaitGroup wrapper for the cluster nodes
type mockNodeWg struct {
	sync.WaitGroup
	count int64
}

func (wg *mockNodeWg) Add(delta int) {
	wg.WaitGroup.Add(delta)
}

func (wg *mockNodeWg) Done() {
	wg.WaitGroup.Done()
	atomic.AddInt64(&wg.count, 1)
}

func (wg *mockNodeWg) getDone() int64 {
	return atomic.LoadInt64(&wg.count)
}

func (wg *mockNodeWg) resetDone() {
	atomic.StoreInt64(&wg.count, 0)
}

// mockCluster represents a mock IBFT cluster
type mockCluster struct {
	nodes []*IBFT           // references to the nodes in the cluster
	ctxs  []mockNodeContext // context handlers for the nodes in the cluster

	wg mockNodeWg
}

func (m *mockCluster) runSequence(height uint64) {
	m.wg.resetDone()

	for nodeIndex, node := range m.nodes {
		m.wg.Add(1)

		go func(
			ctx context.Context,
			node *IBFT,
			height uint64,
		) {
			defer func() {
				m.wg.Done()
			}()

			// Start the main run loop for the node
			node.RunSequence(ctx, height)
		}(m.ctxs[nodeIndex].ctx, node, height)
	}
}

// awaitCompletion waits for completion of all
// nodes in the cluster
func (m *mockCluster) awaitCompletion() {
	// Wait for all main run loops to signalize
	// that they're finished
	m.wg.Wait()
}

// forceShutdown sends a stop signal to all running nodes
// in the cluster, and awaits their completion
func (m *mockCluster) forceShutdown() {
	// Send a stop signal to all the nodes
	for _, ctx := range m.ctxs {
		ctx.cancelFn()
	}

	// Wait for all the nodes to finish
	m.awaitCompletion()
}

// awaitNCompletions awaits completion of the current sequence
// for N nodes in the cluster
func (m *mockCluster) awaitNCompletions(
	ctx context.Context,
	count int64,
) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"await exceeded timeout for %d nodes",
				count,
			)
		default:
			if m.wg.getDone() >= count {
				return nil
			}
		}
	}
}

// pushMessage imitates a message passing service,
// it relays a message to all nodes in the network
func (m *mockCluster) pushMessage(message *proto.Message) {
	for _, node := range m.nodes {
		node.AddMessage(message)
	}
}

// areAllNodesOnRound checks to make sure all nodes
// are on the same specified round
func (m *mockCluster) areAllNodesOnRound(round uint64) bool { //nolint:unused
	for _, node := range m.nodes {
		if node.state.getRound() != round {
			return false
		}
	}

	return true
}

// setBaseTimeout sets the base timeout for rounds
func (m *mockCluster) setBaseTimeout(timeout time.Duration) {
	for _, node := range m.nodes {
		node.baseRoundTimeout = timeout
	}
}

// generateNodeAddresses generates dummy node addresses
func generateNodeAddresses(count uint64) [][]byte {
	addresses := make([][]byte, count)

	for index := range addresses {
		addresses[index] = []byte(fmt.Sprintf("node %d", index))
	}

	return addresses
}

// buildBasicPreprepareMessage builds a simple preprepare message
func buildBasicPreprepareMessage(
	proposal []byte,
	proposalHash []byte,
	certificate *proto.RoundChangeCertificate,
	from []byte,
	view *proto.View,
) *proto.Message {
	return &proto.Message{
		View: view,
		From: from,
		Type: proto.MessageType_PREPREPARE,
		Payload: &proto.Message_PreprepareData{
			PreprepareData: &proto.PrePrepareMessage{
				Proposal:     proposal,
				Certificate:  certificate,
				ProposalHash: proposalHash,
			},
		},
	}
}

// buildBasicPrepareMessage builds a simple prepare message
func buildBasicPrepareMessage(
	proposalHash,
	from []byte,
	view *proto.View,
) *proto.Message {
	return &proto.Message{
		View: view,
		From: from,
		Type: proto.MessageType_PREPARE,
		Payload: &proto.Message_PrepareData{
			PrepareData: &proto.PrepareMessage{
				ProposalHash: proposalHash,
			},
		},
	}
}

// buildBasicCommitMessage builds a simple commit message
func buildBasicCommitMessage(
	proposalHash,
	committedSeal,
	from []byte,
	view *proto.View,
) *proto.Message {
	return &proto.Message{
		View: view,
		From: from,
		Type: proto.MessageType_COMMIT,
		Payload: &proto.Message_CommitData{
			CommitData: &proto.CommitMessage{
				ProposalHash:  proposalHash,
				CommittedSeal: committedSeal,
			},
		},
	}
}

// buildBasicRoundChangeMessage builds a simple round change message
func buildBasicRoundChangeMessage(
	proposal []byte,
	certificate *proto.PreparedCertificate,
	view *proto.View,
	from []byte,
) *proto.Message {
	return &proto.Message{
		View: view,
		From: from,
		Type: proto.MessageType_ROUND_CHANGE,
		Payload: &proto.Message_RoundChangeData{
			RoundChangeData: &proto.RoundChangeMessage{
				LastPreparedProposedBlock: proposal,
				LatestPreparedCertificate: certificate,
			},
		},
	}
}

// maxFaulty returns the maximum number of allowed
// faulty nodes
func maxFaulty(nodeCount uint64) uint64 {
	return (nodeCount - 1) / 3
}

// quorum returns the minimum number of
// required nodes to reach quorum
func quorum(numNodes uint64) uint64 {
	switch maxFaulty(numNodes) {
	case 0:
		return numNodes
	default:
		return uint64(math.Ceil(2 * float64(numNodes) / 3))
	}
}
