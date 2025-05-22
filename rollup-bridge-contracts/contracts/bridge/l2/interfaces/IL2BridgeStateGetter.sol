// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

interface IL2BridgeStateGetter {
  function l1MessageHash() external view returns (bytes32);

  function getL2ToL1Root() external view returns (bytes32);

  function getLatestDepositNonce() external view returns (uint256);
}
