// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IL1Bridge } from "./IL1Bridge.sol";
import { INilGasPriceOracle } from "./INilGasPriceOracle.sol";

interface IL1ETHBridge is IL1Bridge {
  struct DepositMessageParams {
    address depositorAddress;
    uint256 depositAmount;
    address payable l2DepositRecipient;
    address l2FeeRefundAddress;
    uint256 l2GasLimit;
    uint256 userMaxFeePerGas;
    uint256 userMaxPriorityFeePerGas;
    INilGasPriceOracle.FeeCreditData feeCreditData;
  }

  // Group related variables into a struct to reduce stack usage
  struct DepositETHMessageData {
    address depositorAddress;
    uint256 depositAmount;
    address payable l2DepositRecipient;
    address l2FeeRefundAddress;
    INilGasPriceOracle.FeeCreditData feeCreditData;
    bytes depositMessage;
  }

  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Thrown when the function selector for finalizing the deposit is invalid
  error ErrorInvalidFinaliseDepositFunctionSelector();

  event DepositCancelled(bytes32 indexed messageHash, address indexed cancelledDepositRecipient, uint256 amount);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  function l2EthAddress() external view returns (address);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATING FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /**
   * @notice Initiates the ETH to the nil-chain. for a specified recipient.
   * @param depositAmount The amount of ETH to deposit.
   * @param l2DepositRecipient The recipient address to receive the token in nil-chain.
   * @param l2GasLimit The gas limit required to complete the deposit on nil-chain.
   * @param userMaxFeePerGas User-defined optional maxFeePerGas
   * @param userMaxPriorityFeePerGas User-defined optional maxPriorityFeePerGas
   */
  function depositETH(
    uint256 depositAmount,
    address payable l2DepositRecipient,
    address l2FeeRefundAddress,
    uint256 l2GasLimit,
    uint256 userMaxFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) external payable;

  /**
   * @notice Deposits ETH to the nil-chain for a specified recipient and calls a function on the recipient's
   * contract.
   * @param depositorAddress The address of depositor who has initiated deposit via L1BridgeRouter
   * @param depositAmount The amount of ETH to deposit.
   * @param l2DepositRecipient The recipient address to receive the ETH in nil-chain.
   * @param l2FeeRefundAddress The recipient address to receive the ETH in nil-chain.
   * @param l2GasLimit The gas limit required to complete the deposit on nil-chain.
   * @param userMaxFeePerGas User-defined optional maxFeePerGas
   * @param userMaxPriorityFeePerGas User-defined optional maxPriorityFeePerGas
   */
  function depositETHViaRouter(
    address depositorAddress,
    uint256 depositAmount,
    address payable l2DepositRecipient,
    address l2FeeRefundAddress,
    uint256 l2GasLimit,
    uint256 userMaxFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) external payable;

  function finaliseETHWithdrawal(address l1WithdrawalRecipient, uint256 withdrawalAmount) external payable;
}
