package l1

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
)

type L1Contract interface {
	SubscribeToEvents(ctx context.Context, sink chan<- *L1MessageSent) (event.Subscription, error)
	GetEventsFromBlockRange(ctx context.Context, from uint64, to *uint64) ([]*L1MessageSent, error)
}

type l1ContractWrapper struct {
	impl *L1

	// addresses of the token bridges (ETH, ERC20, etc.) deployed on L2, used to filter events
	l2BridgeAddresses []common.Address
}

var _ L1Contract = (*l1ContractWrapper)(nil)

func NewL1ContractWrapper(
	ethClient EthClient,
	l1ContractAddr string,
	bridges []string,
	logger logging.Logger,
) (*l1ContractWrapper, error) {
	addr := common.HexToAddress(l1ContractAddr)
	impl, err := NewL1(addr, ethClient)
	if err != nil {
		return nil, err
	}

	wrapper := &l1ContractWrapper{
		impl: impl,
	}
	for _, bridge := range bridges {
		wrapper.l2BridgeAddresses = append(wrapper.l2BridgeAddresses, common.HexToAddress(bridge))
	}

	if len(wrapper.l2BridgeAddresses) == 0 {
		logger.Warn().Msg("No L2 bridge addresses provided, all events will be fetched")
	}

	return wrapper, nil
}

func (w *l1ContractWrapper) SubscribeToEvents(
	ctx context.Context,
	sink chan<- *L1MessageSent,
) (event.Subscription, error) {
	return w.impl.WatchMessageSent(
		&bind.WatchOpts{Context: ctx},
		sink,
		nil, // any sender (for now)
		w.l2BridgeAddresses,
		nil, // any nonce
	)
}

func (w *l1ContractWrapper) GetEventsFromBlockRange(
	ctx context.Context,
	from uint64,
	to *uint64,
) ([]*L1MessageSent, error) {
	iter, err := w.impl.FilterMessageSent(
		&bind.FilterOpts{
			Start:   from,
			End:     to,
			Context: ctx,
		},
		nil, // any sender (for now)
		w.l2BridgeAddresses,
		nil, // any nonce
	)
	if err != nil {
		return nil, err
	}

	// oclaw: it is not expected to be too much events here, so getting rid of iterator for simplicity
	// can be changed though
	var ret []*L1MessageSent
	for iter.Next() {
		ret = append(ret, iter.Event)
	}

	return ret, iter.Error()
}
