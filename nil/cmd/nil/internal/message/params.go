package message

import (
	"github.com/NilFoundation/nil/nil/internal/types"
)

const (
	shardIdFlag     = "shard-id"
	bounceFlag      = "bounce"
	bounceFlagShort = "b"
	kindFlag        = "kind"
	feeCreditFlag   = "fee-credit"
	forwardKindFlag = "fwd-kind"
	toFlag          = "to"
	refundToFlag    = "refund-to"
	bounceToFlag    = "bounce-to"
	valueFlag       = "value"
	dataFlag        = "data"
)

var params = &messageParams{}

type messageParams struct {
	shardId types.ShardId
}
