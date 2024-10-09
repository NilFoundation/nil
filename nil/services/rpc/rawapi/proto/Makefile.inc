.PHONY: pb_rawapi
pb_rawapi: nil/services/rpc/rawapi/pb/block.pb.go nil/services/rpc/rawapi/pb/message.pb.go nil/services/rpc/rawapi/pb/call.pb.go

nil/services/rpc/rawapi/pb/block.pb.go: nil/services/rpc/rawapi/proto/block.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/block.proto

nil/services/rpc/rawapi/pb/message.pb.go: nil/services/rpc/rawapi/proto/message.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/message.proto

nil/services/rpc/rawapi/pb/call.pb.go: nil/services/rpc/rawapi/proto/call.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/call.proto
