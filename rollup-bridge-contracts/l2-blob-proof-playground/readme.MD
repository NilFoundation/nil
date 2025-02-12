# DataProof Integration Guide

## Data Structures

### KZGProofResponse

This struct holds the response for a KZG proof generation.

```go
type KZGProofResponse struct {
    Point kzg4844.Point
    Claim kzg4844.Claim
    Proof kzg4844.Proof
}
```

### DataProofResponse

This struct holds the response for a data proof generation.

```go
type DataProofResponse struct {
    Point         kzg4844.Point
    Claim         [32]byte
    KZGCommitment kzg4844.Commitment
    KZGProof      kzg4844.Proof
    DataProof     []byte
}
```

## Overview

This guide provides detailed instructions on how to integrate and use the DataProof functionality in your Go project.  
The DataProof functionality allows you to generate and verify data proofs from blobs using KZG proofs.

## Prerequisites

- Go 1.23.0 or later

## Installation

First, ensure you have the necessary packages installed

```sh
go mod tidy
```

## Proof Generation Logic

[Proof-Generation](./proof/proof-generation.MD)

## Usage

### 1. Generate a Random Blob

Use the `GenerateRandomBlob` function from the `blobutil` package to generate a random blob.

```go
package main

import (
    "fmt"
    "github.com/ethereum/go-ethereum/crypto/kzg4844"
    "nil-blob-playground/blob-util"
)

func main() {
    blob, err := blobutil.GenerateRandomBlob()
    if err != nil {
        panic(err)
    }
    fmt.Printf("Generated Blob: %v\n", blob)
}
```

### 2. Generate Data Proof from a Blob

Use the `GenerateDataProofFromBlob` function from the `proof` package to generate a data proof from a blob.

```go
package main

import (
    "fmt"
    "github.com/ethereum/go-ethereum/crypto/kzg4844"
    "nil-blob-playground/proof"
    "nil-blob-playground/blob-util"
)

func main() {
    blob, err := blobutil.GenerateRandomBlob()
    if err != nil {
        panic(err)
    }

    dataProofResponse, err := proof.GenerateDataProofFromBlob(blob)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Generated Data Proof: %x\n", dataProofResponse.DataProof)
}
```

### 3. Generate Data Proofs from a Sidecar

Use the `GenerateDataProofFromSidecar` function to generate data proofs for all blobs in a sidecar.

```go
package main

import (
    "fmt"
    gethTypes "github.com/ethereum/go-ethereum/core/types"
    "nil-blob-playground/proof"
    "nil-blob-playground/blob-util"
)

func main() {
    // Generate a sidecar with multiple blobs
    sidecar, err := GenerateSideCarWithMultipleBlobs(3)
    if err != nil {
        panic(err)
    }

    dataProofResponses, err := proof.GenerateDataProofFromSidecar(sidecar)
    if err != nil {
        panic(err)
    }

    for i, dataProofResponse := range dataProofResponses {
        fmt.Printf("Data Proof %d: %x\n", i, dataProofResponse.DataProof)
    }
}

func GenerateSideCarWithMultipleBlobs(blobCount int) (*gethTypes.BlobTxSidecar, error) {
    var blobs []*kzg4844.Blob
    for i := 0; i < blobCount; i++ {
        blob, err := blobutil.GenerateRandomBlob()
        if err != nil {
            return nil, err
        }
        blobs = append(blobs, blob)
    }
    sidecar, err := blobutil.MakeSidecar(blobs...)
    if err != nil {
        return nil, err
    }
    return sidecar, nil
}
```

### 4. Verify Data Proof

Use the `VerifyProof` function from the `kzg4844` package to verify the generated data proof.

```go
package main

import (
    "fmt"
    "github.com/ethereum/go-ethereum/crypto/kzg4844"
    "nil-blob-playground/proof"
    "nil-blob-playground/blob-util"
)

func main() {
    blob, err := blobutil.GenerateRandomBlob()
    if err != nil {
        panic(err)
    }

    dataProofResponse, err := proof.GenerateDataProofFromBlob(blob)
    if err != nil {
        panic(err)
    }

    kzgCommitment, err := kzg4844.BlobToCommitment(blob)
    if err != nil {
        panic(err)
    }

    err = kzg4844.VerifyProof(kzgCommitment, dataProofResponse.Point, dataProofResponse.Claim, dataProofResponse.KZGProof)
    if err != nil {
        fmt.Printf("Verification failed: %v\n", err)
    } else {
        fmt.Println("Verification succeeded")
    }
}
```

## Functions

### `GenerateRandomBlob`

Generates a random blob.

**Usage:**

```go
blob, err := blobutil.GenerateRandomBlob()
```

### `GenerateDataProofFromBlob`

Generates a data proof from a given blob.

**Usage:**

```go
dataProofResponse, err := proof.GenerateDataProofFromBlob(blob)
```

### `GenerateDataProofFromSidecar`

Generates data proofs for all blobs in a sidecar.

**Usage:**

```go
dataProofResponses, err := proof.GenerateDataProofFromSidecar(sidecar)
```

### `VerifyProof`

Verifies the generated data proof.

**Usage:**

```go
err := kzg4844.VerifyProof(kzgCommitment, point, claim, kzgProof)
```

## Conclusion

This guide provides a comprehensive overview of how to generate and verify data proofs using the `nil-blob-playground` package.   
By following the steps outlined above, you can integrate data proof functionality into your Go project seamlessly.
