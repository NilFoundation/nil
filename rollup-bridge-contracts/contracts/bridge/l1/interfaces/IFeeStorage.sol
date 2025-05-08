// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

/// @title IFeeStorage
/// @notice Interface for the INilGasPriceOracle contract which also used by sync_committee.
interface IFeeStorage {
  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/
  /// @notice Return the latest known maxFeePerGas and maxPriorityFeePerGas from nil-chain
  function getOracleFee() external view returns (uint256, uint256);

  function maxFeePerGas() external view returns (uint256);

  function maxPriorityFeePerGas() external view returns (uint256);

  /*//////////////////////////////////////////////////////////////////////////
                             RESTRICTED FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice set the maxFeePerGas & maxPriorityFeePerGas from nil-chain
  function setOracleFee(uint256 maxFeePerGas, uint256 maxPriorityFeePerGas) external;
}
