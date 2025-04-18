// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { L1BridgeMessengerEvents } from "../../libraries/L1BridgeMessengerEvents.sol";

interface INilGasPriceOracle is IERC165 {
  /// @dev Invalid owner address.
  error ErrorInvalidOwner();

  /// @dev Invalid default admin address.
  error ErrorInvalidDefaultAdmin();

  error ErrorNotAuthorised();

  function getImplementation() external view returns (address);

  /// @notice set the maxFeePerGas & maxPriorityFeePerGas from nil-chain
  function setFeePerGas(uint256 newMaxFeePerGas, uint256 newMaxPriorityFeePerGas) external;

  /// @notice set the maxFeePerGas from nil-chain
  function setMaxFeePerGas(uint256 maxFeePerGas) external;

  /// @notice Return the latest known maxFeePerGas from nil-chain
  function maxFeePerGas() external view returns (uint256);

  /// @notice set the maxPriorityFeePerGas from nil-chain
  function setMaxPriorityFeePerGas(uint256 maxPriorityFeePerGas) external;

  /// @notice Return the latest known maxPriorityFeePerGas from nil-chain
  function maxPriorityFeePerGas() external view returns (uint256);

  /// @notice Return the latest known maxFeePerGas, maxPriorityFeePerGas from nil-chain
  function getFeeData() external view returns (uint256, uint256);

  function computeFeeCredit(
    uint256 gasLimit,
    uint256 userMaxFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) external view returns (L1BridgeMessengerEvents.FeeCreditData memory);
}
