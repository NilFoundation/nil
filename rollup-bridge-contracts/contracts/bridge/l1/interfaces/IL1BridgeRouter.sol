// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

interface IL1BridgeRouter {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @dev Invalid owner address.
  error ErrorInvalidOwner();

  error ErrorUnauthorizedCaller();

  error ErrorWETHTokenNotSupported();

  /// @dev Invalid default admin address.
  error ErrorInvalidDefaultAdmin();

  error ErrorERC20PullFailed();

  error ErrorInvalidTokenAddress();

  error ErrorInvalidL1ERC20BridgeAddress();

  error ErrorInvalidMessageType();

  error ErrorEmptyDeposit();

  error ErrorInvalidNilGasLimit();

  error ErrorInvalidL1ETHBridgeAddress();

  error ErrorInvalidL2DepositRecipient();

  error ErrorInvalidMessenger();

  error ErrorInvalidERC20Bridge();

  /**
   * @notice Emitted when the L1ERC20Bridge address is set.
   * @param oldERC20Bridge The previous L1ERC20Bridge address.
   * @param newERC20Bridge The new L1ERC20Bridge address.
   */
  event ERC20BridgeSet(address indexed oldERC20Bridge, address indexed newERC20Bridge);

  /**
   * @notice Emitted when the L1ETHBridge address is set.
   * @param oldETHBridge The previous L1ETHBridge address.
   * @param newETHBridge The new L1ETHBridge address.
   */
  event ETHBridgeSet(address indexed oldETHBridge, address indexed newETHBridge);

  /**
   * @notice Emitted when the L1BridgeMessenger address is set.
   * @param oldMessenger The previous L1BridgeMessenger address.
   * @param newMessenger The new L1BridgeMessenger address.
   */
  event MessengerSet(address indexed oldMessenger, address indexed newMessenger);

  /**
   * @notice Emitted when the WETH address is set.
   * @param oldWETH The previous WETH address.
   * @param newWETH The new WETH address.
   */
  event WETHSet(address indexed oldWETH, address indexed newWETH);

  /**
   * @notice Returns the L2 token address corresponding to the given L1 token address.
   * @param l1TokenAddress The address of the L1 token.
   * @return The address of the corresponding L2 token.
   */
  function getL2TokenAddress(address l1TokenAddress) external view returns (address);

  /**
   * @notice Returns the address of the L1ERC20Bridge contract.
   * @return The address of the L1ERC20Bridge contract.
   */
  function erc20Bridge() external view returns (address);

  /**
   * @notice Returns the address of the L1ETHBridge contract.
   * @return The address of the L1ETHBridge contract.
   */
  function ethBridge() external view returns (address);

  /**
   * @notice Returns the address of the L1WETH contract.
   * @return The address of the L1WETH contract.
   */
  function wethAddress() external view returns (address);

  function getImplementation() external view returns (address);

  /**
   * @notice Sets the address of the L1ERC20Bridge contract.
   * @param newERC20Bridge The new address of the L1ERC20Bridge contract.
   */
  function setERC20Bridge(address newERC20Bridge) external;

  /**
   * @notice Sets the address of the L1ETHBridge contract.
   * @param ethBridge The new address of the L1ETHBridge contract.
   */
  function setETHBridge(address ethBridge) external;

  /**
   * @notice Sets the address of the L1BridgeMessenger contract.
   * @param messenger The new address of the L1BridgeMessenger contract.
   */
  function setMessenger(address messenger) external;

  /**
   * @notice Sets the address of the wethAddress
   * @param wethAddress The new wethAddress on L1
   */
  function setWETHAddress(address wethAddress) external;

  /**
   * @notice Pulls ERC20 tokens from the sender to the bridge contract.
   * @dev This function can only be called by authorized bridge contracts.
   * @dev All bridge contracts must have reentrancy guard to prevent potential attack through this function.
   * @dev L1Bridge Contract - L1ERC20Bridge will call this function to let the router pull the tokens from the
   * depositor to the corresponding bridge address.
   * @dev This function is invoked only when the depositor calls L1BridgeRouter for bridging.
   * @param sender The address of the sender from whom the tokens will be pulled.
   * @param token The address of the ERC20 token to be pulled.
   * @param amount The amount of tokens to be pulled.
   * @return The actual amount of tokens pulled.
   */
  function pullERC20(address sender, address token, uint256 amount) external returns (uint256);

  /**
   * @notice Initiates the ERC20 tokens to the nil-chain. for a specified recipient.
   * @param l1Token The address of the ERC20 in L1 token to deposit.
   * @param l2DepositRecipient The recipient address to receive the token in nil-chain.
   * @param depositAmount The amount of tokens to deposit.
   * @param l2GasLimit The gas limit required to complete the deposit on nil-chain.
   */
  function depositERC20(
    address l1Token,
    uint256 depositAmount,
    address l2DepositRecipient,
    address l2FeeRefundAddress,
    uint256 l2GasLimit,
    uint256 userMaxFeePerGas, // User-defined optional maxFeePerGas
    uint256 userMaxPriorityFeePerGas // User-defined optional maxPriorityFeePerGas
  ) external payable;

  /**
   * @notice Initiates the ETH to the nil-chain. for a specified recipient.
   * @param l2DepositRecipient The recipient address to receive the token in nil-chain.
   * @param depositAmount The amount of ETH to deposit.
   * @param l2GasLimit The gas limit required to complete the deposit on nil-chain..
   */
  function depositETH(
    uint256 depositAmount,
    address payable l2DepositRecipient,
    address l2FeeRefundAddress,
    uint256 l2GasLimit,
    uint256 userMaxFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) external payable;

  function cancelDeposit(bytes32 messageHash) external payable;

  function claimFailedDeposit(bytes32 messageHash, bytes32[] memory claimProof) external;

  /**
   * @notice Pauses or unpauses the contract.
   * @dev This function allows the owner to pause or unpause the contract.
   * @param statusValue The pause status to update.
   */
  function setPause(bool statusValue) external;

  /**
   * @notice transfers ownership to the newOwner.
   * @dev This function revokes the `OWNER_ROLE` from the current owner, calls `acceptOwnership` using
   * OwnableUpgradeable's `transferOwnership` transfer the owner rights to newOwner
   * @param newOwner The address of the new owner.
   */
  function transferOwnershipRole(address newOwner) external;
}
