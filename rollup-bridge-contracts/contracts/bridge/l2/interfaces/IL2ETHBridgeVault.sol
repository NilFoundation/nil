// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";

interface IL2ETHBridgeVault is IERC165 {
    error ErrorInvalidL2ETHBridge();
    error ErrorCallerNotL2ETHBridge();
    error ErrorInvalidRecipientAddress();
    error ErrorInvalidTransferAmount();
    error ErrorInsufficientVaultBalance();
    error ErrorUnauthorisedFunding();
    /// @dev Invalid owner address.
    error ErrorInvalidOwner();

    /// @dev Invalid default admin address.
    error ErrorInvalidDefaultAdmin();

    /// @dev Invalid address.
    error ErrorInvalidAddress();

    error ErrorETHTransferFailed();

    event L2ETHBridgeSet(address indexed l2ETHBridge, address indexed newL2ETHBridge);

    function setL2ETHBridge(address l2EthBridgeAddress) external;

    /// @notice Transfers ETH to a recipient, only callable by the L2ETHBridge contract
    /// @param recipient The address of the recipient
    /// @param amount The amount of ETH to transfer
    function transferETH(address payable recipient, uint256 amount) external;
}
