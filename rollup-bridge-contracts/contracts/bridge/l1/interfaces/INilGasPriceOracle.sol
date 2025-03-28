// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";

interface INilGasPriceOracle is IERC165 {
    /// @dev Invalid owner address.
    error ErrorInvalidOwner();

    /// @dev Invalid default admin address.
    error ErrorInvalidDefaultAdmin();

    struct FeeCreditData {
        uint256 nilGasLimit;
        uint256 maxFeePerGas;
        uint256 maxPriorityFeePerGas;
        uint256 feeCredit;
    }

    /// @notice set the maxFeePerGas for nil-chain
    function setMaxFeePerGas(uint256 maxFeePerGas) external;

    /// @notice Return the latest known maxFeePerGas for nil-chain
    function maxFeePerGas() external view returns (uint256);

    /// @notice set the maxPriorityFeePerGas for nil-chain
    function setMaxPriorityFeePerGas(uint256 maxPriorityFeePerGas) external;

    /// @notice Return the latest known maxPriorityFeePerGas for nil-chain
    function maxPriorityFeePerGas() external view returns (uint256);

    /// @notice Return the latest known maxFeePerGas, maxPriorityFeePerGas for nil-chain
    function getFeeData() external view returns (uint256, uint256);

    function computeFeeCredit(
        uint256 gasLimit,
        uint256 userMaxFeePerGas,
        uint256 userMaxPriorityFeePerGas
    )
        external
        view
        returns (FeeCreditData memory);
}
