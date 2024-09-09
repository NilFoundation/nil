package logging

const (
	// FieldError can be used instead of Err(err) if you have only the error message string.
	FieldError = "err"

	FieldComponent = "component"
	FieldShardId   = "shardId"
	FieldChainId   = "chainId"

	FieldDuration = "duration"
	FieldUrl      = "url"
	FieldReqId    = "reqId"

	FieldRpcMethod = "rpcMethod"
	FieldRpcParams = "rpcParams"

	FieldP2PIdentity = "p2pIdentity"
	FieldPeerId      = "peerId"
	FieldTopic       = "topic"
	FieldProtocolID  = "protocolId"
	FieldTcpPort     = "tcpPort"
	FieldQuicPort    = "quicPort"

	FieldMessageHash  = "msgHash"
	FieldMessageSeqno = "msgSeqno"
	FieldMessageFrom  = "msgFrom"
	FieldMessageTo    = "msgTo"
	FieldFullMessage  = "msg"

	FieldAccountAddress = "accountAddress"
	FieldAccountSeqno   = "accountSeqno"

	FieldBlockHash   = "blockHash"
	FieldBlockNumber = "blockNumber"

	FieldCurrencyId = "CurrencyId"
)
