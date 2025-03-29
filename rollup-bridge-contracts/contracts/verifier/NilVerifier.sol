// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { INilVerifier } from "../interfaces/INilVerifier.sol";

contract NilVerifier is INilVerifier {
    /// @inheritdoc INilVerifier
    function verify(bytes calldata validityProof, bytes calldata publicInput) external view override { }
}
