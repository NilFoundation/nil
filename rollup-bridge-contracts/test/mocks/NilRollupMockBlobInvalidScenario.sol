// SPDX-License-Identifier: MIT
pragma solidity 0.8.27;

import { NilRollupMockBlob } from "./NilRollupMockBlob.sol";

contract NilRollupMockBlobInvalidScenario is NilRollupMockBlob {
    // used for test to assert the revert on invalidPublicInput bytes assertion
    function generatePublicInputForValidityProofVerification(
        string memory batchIndex,
        PublicDataInfo calldata publicDataInfo
    )
        public
        view
        override
        returns (bytes memory)
    {
        return hex"";
    }
}
