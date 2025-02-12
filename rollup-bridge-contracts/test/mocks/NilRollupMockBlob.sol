// SPDX-License-Identifier: MIT
pragma solidity 0.8.27;

import { NilRollup } from "../../contracts/NilRollup.sol";

contract NilRollupMockBlob is NilRollup {
    mapping(uint256 => bytes32) private mockBlobHashes;
    uint256[] public mockBlobKeys;

    function setBlobVersionedHash(uint256 index, bytes32 hash) public {
        if (mockBlobHashes[index] == bytes32(0)) {
            mockBlobKeys.push(index);
        }
        mockBlobHashes[index] = hash;
    }

    function clearMockBlobHashes() public {
        for (uint256 i = 0; i < mockBlobKeys.length; i++) {
            delete mockBlobHashes[mockBlobKeys[i]];
        }
        delete mockBlobKeys;
    }

    function getBlobHash(uint256 index) public view override returns (bytes32) {
        return mockBlobHashes[index];
    }
}
