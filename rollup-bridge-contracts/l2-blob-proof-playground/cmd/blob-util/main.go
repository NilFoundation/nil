package main

import (
	"fmt"
	blobutil "l2-blob-proof-playground/blob-util"
)

// go run cmd/blob-util/main.go
func main() {
	// Test generateRandBlob function
	blob, err := blobutil.GenerateRandomBlob()

	if err != nil {
		panic(err)
	}

	//fmt.Printf("Generated Blob: %x\n", blob)
	fmt.Printf("Blob length is %d\n", len(*blob))
	fmt.Printf("Blob capacity is %d\n", cap(*blob))
	fmt.Printf("Blob size in KB is %d\n", len(*blob)/1024)

	// Test generateRandFieldElement function
	fieldElement := blobutil.GenerateRandomFieldElement()
	fmt.Printf("Generated Field Element: %x\n", fieldElement)
}
