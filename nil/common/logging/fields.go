package logging

const (
	// FieldError can be used instead of Err(err) if you have only the error message string.
	FieldError = "err"

	FieldComponent = "component"
	FieldShardId   = "shardId"

	FieldDuration = "duration"
	FieldUrl      = "url"
	FieldReqId    = "reqId"

	FieldRpcPort   = "rpcPort"
	FieldRpcMethod = "rpcMethod"
	FieldRpcParams = "rpcParams"
	FieldRpcResult = "rpcResult"

	FieldP2PIdentity = "p2pIdentity"
	FieldPeerId      = "peerId"
	FieldTopic       = "topic"
	FieldProtocolID  = "protocolId"

	FieldTransactionHash  = "txnHash"
	FieldTransactionSeqno = "txnSeqno"
	FieldTransactionFrom  = "txnFrom"
	FieldTransactionTo    = "txnTo"
	FieldTransactionFlags = "txnFlags"
	FieldFullTransaction  = "txn"

	FieldAccountSeqno = "accountSeqno"

	FieldBlockHash          = "blockHash"
	FieldBlockMainShardHash = "blockMainShardHash"
	FieldBlockNumber        = "blockNumber"
	FieldBatchId            = "batchId"
	FieldStateRoot          = "stateRoot"

	FieldTaskId         = "taskId"
	FieldTaskParentId   = "taskParentId"
	FieldTaskType       = "taskType"
	FieldTaskExecutorId = "taskExecutorId"

	FieldTokenId = "TokenId"

	FieldPublicKey = "publicKey"
	FieldSignature = "signature"
	FieldHeight    = "height"
	FieldRound     = "round"
	FieldType      = "type"

	FieldClientType    = "clientType"
	FieldClientVersion = "clientVersion"
	FieldUid           = "uid"

	FieldStoreToClickhouse = "storeToClickhouse"

	FieldHostName    = "_HOSTNAME"
	FieldSystemdUnit = "_SYSTEMD_UNIT"
)
