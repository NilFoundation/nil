.PHONY: pb_rawapi
pb_rawapi: nil/services/rpc/rawapi/pb/block.pb.go

nil/services/rpc/rawapi/pb/block.pb.go: nil/services/rpc/rawapi/proto/block.proto
	protoc --go_out=nil/services/rpc/rawapi/ nil/services/rpc/rawapi/proto/block.proto
