.PHONY: pb_rawapi
pb_rawapi: \
	nil/services/rpc/rawapi/pb/account.pb.go \
	nil/services/rpc/rawapi/pb/block.pb.go \
	nil/services/rpc/rawapi/pb/bootstrap_config.pb.go \
	nil/services/rpc/rawapi/pb/transaction.pb.go \
	nil/services/rpc/rawapi/pb/call.pb.go \
	nil/services/rpc/rawapi/pb/common.pb.go \
	nil/services/rpc/rawapi/pb/send.pb.go \
	nil/services/rpc/rawapi/pb/system.pb.go

nil/services/rpc/rawapi/pb/account.pb.go: nil/services/rpc/rawapi/proto/account.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/account.proto

nil/services/rpc/rawapi/pb/block.pb.go: nil/services/rpc/rawapi/proto/block.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/block.proto

nil/services/rpc/rawapi/pb/bootstrap_config.pb.go: nil/services/rpc/rawapi/proto/bootstrap_config.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/bootstrap_config.proto

nil/services/rpc/rawapi/pb/transaction.pb.go: nil/services/rpc/rawapi/proto/transaction.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/transaction.proto

nil/services/rpc/rawapi/pb/call.pb.go: nil/services/rpc/rawapi/proto/call.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/call.proto

nil/services/rpc/rawapi/pb/common.pb.go: nil/services/rpc/rawapi/proto/common.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/common.proto

nil/services/rpc/rawapi/pb/send.pb.go: nil/services/rpc/rawapi/proto/send.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/send.proto

nil/services/rpc/rawapi/pb/system.pb.go: nil/services/rpc/rawapi/proto/system.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/system.proto
