.PHONY: pb_ibft
pb_ibft: nil/go-ibft/messages/proto/messages.pb.go

nil/go-ibft/messages/proto/messages.pb.go: nil/go-ibft/messages/proto/messages.proto
	protoc --go_out=nil/go-ibft/ nil/go-ibft/messages/proto/messages.proto
