export interface PublicDataInfo {
    placeholder1: string;
    placeholder2: string;
}

export interface BatchInfo {
    batchIndex: string;
    isCommitted: boolean;
    isFinalized: boolean;
    versionedHashes: string[];
    oldStateRoot: string;
    newStateRoot: string;
    dataProofs: string[];
    validityProof: string;
    publicDataInputs: PublicDataInfo;
    blobCount: number;
}
