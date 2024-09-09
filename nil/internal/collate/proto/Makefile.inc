.PHONY: pb_collator
pb_collator: nil/internal/collate/pb/block.pb.go

nil/internal/collate/pb/block.pb.go: nil/internal/collate/proto/block.proto
	protoc --go_out=nil/internal/collate nil/internal/collate/proto/block.proto
