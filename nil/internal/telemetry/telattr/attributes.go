package telattr

import (
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const ServiceNameKey = string(semconv.ServiceNameKey)

func ShardId(id types.ShardId) attribute.KeyValue {
	return attribute.Int(logging.FieldShardId, int(id))
}

func P2PIdentity(id peer.ID) attribute.KeyValue {
	return attribute.Stringer(logging.FieldP2PIdentity, id)
}

func PeerId(id peer.ID) attribute.KeyValue {
	return attribute.Stringer(logging.FieldPeerId, id)
}

func ProtocolId(id protocol.ID) attribute.KeyValue {
	return attribute.String(logging.FieldProtocolID, string(id))
}

func Topic(topic string) attribute.KeyValue {
	return attribute.String(logging.FieldTopic, topic)
}

func Type(t string) attribute.KeyValue {
	return attribute.String(logging.FieldType, t)
}

func RpcMethod(method string) attribute.KeyValue {
	return attribute.String(logging.FieldRpcMethod, method)
}
