// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { IRelayMessage } from "./IRelayMessage.sol";

interface INilGasPriceOracle is IERC165 {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS   
    //////////////////////////////////////////////////////////////////////////*/

  error ErrorInvalidMaxFeePerGas();

  error ErrorInvalidMaxPriorityFeePerGas();

  error ErrorInvalidGasLimitForFeeCredit();

  /// @dev Invalid owner address.
  error ErrorInvalidOwner();

  /// @dev Invalid default admin address.
  error ErrorInvalidDefaultAdmin();

  error ErrorNotAuthorised();

  /*//////////////////////////////////////////////////////////////////////////
                             EVENTS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Emitted when oracleFee is updated.
  /// @param maxFeePerGas The maxFeePerGas updated value.
  /// @param maxPriorityFeePerGas The maxPriorityFeePerGas updated updated.
  event OracleFeeUpdated(uint256 maxFeePerGas, uint256 maxPriorityFeePerGas);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  function getImplementation() external view returns (address);

  function computeFeeCredit(
    uint256 gasLimit,
    uint256 userMaxFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) external view returns (IRelayMessage.FeeCreditData memory);

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
