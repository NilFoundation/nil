import { ethers } from 'hardhat';

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

export const proposerRoleHash = ethers.keccak256(ethers.toUtf8Bytes("PROPOSER_ROLE"));

