// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { NilConstants } from "../../common/libraries/NilConstants.sol";
import { IL1ETHBridge } from "./interfaces/IL1ETHBridge.sol";
import { IL2ETHBridge } from "../l2/interfaces/IL2ETHBridge.sol";
import { IL1BridgeRouter } from "./interfaces/IL1BridgeRouter.sol";
import { IL1Bridge } from "./interfaces/IL1Bridge.sol";
import { IBridge } from "../interfaces/IBridge.sol";
import { IL1BridgeMessenger } from "./interfaces/IL1BridgeMessenger.sol";
import { INilGasPriceOracle } from "./interfaces/INilGasPriceOracle.sol";
import { NilAccessControlUpgradeable } from "../../NilAccessControlUpgradeable.sol";
import { L1BaseBridge } from "./L1BaseBridge.sol";
import { L1BridgeMessengerEvents } from "../libraries/L1BridgeMessengerEvents.sol";

/// @title L1ETHBridge
/// @notice The `L1ETHBridge` contract for ETH bridging from L1.
contract L1ETHBridge is L1BaseBridge, IL1ETHBridge {
  // Define the function selector for finalizeDepositETH as a constant
  bytes4 public constant FINALISE_DEPOSIT_ETH_SELECTOR = IL2ETHBridge.finaliseETHDeposit.selector;

  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @dev Failed to refund ETH for the depositMessage
  error ErrorEthRefundFailed(bytes32 messageHash);

  /// @dev Error due to Zero eth-deposit
  error ErrorZeroEthDeposit();

  /// @dev Error due to invalid l2 recipient address
  error ErrorInvalidL2Recipient();

  /// @dev Error due to invalid L2 GasLimit
  error ErrorInvalidL2GasLimit();

  /*//////////////////////////////////////////////////////////////////////////
                             STATE-VARIABLES   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice address of ETH token on l2
  /// @dev ETH on L2 is an ERC20Token
  address public override l2EthAddress;

  /// @dev The storage slots for future usage.
  uint256[50] private __gap;

  /*//////////////////////////////////////////////////////////////////////////
                             CONSTRUCTOR   
    //////////////////////////////////////////////////////////////////////////*/

  /// @custom:oz-upgrades-unsafe-allow constructor
  /// @notice Constructor for `L1ETHBridge` implementation contract.
  constructor() {
    _disableInitializers();
  }

  /// @notice Initialize the storage of L1ETHBridge.
  /// @param ownerAddress The owner of L1ETHBridge
  /// @param adminAddress The address of admin who is granted DEFAULT_ADMIN role on L1ETHBridge.
  /// @param messengerAddress The address of L1BridgeMessengewethTokenAddress
  /// @param nilGasPriceOracleAddress The address of NilGasPriceOracle on L1
  function initialize(
    address ownerAddress,
    address adminAddress,
    address messengerAddress,
    address nilGasPriceOracleAddress
  ) public initializer {
    // Validate input parameters
    if (ownerAddress == address(0)) {
      revert ErrorInvalidOwner();
    }

    if (adminAddress == address(0)) {
      revert ErrorInvalidDefaultAdmin();
    }

    if (nilGasPriceOracleAddress == address(0)) {
      revert ErrorInvalidNilGasPriceOracle();
    }
    L1BaseBridge.__L1BaseBridge_init(ownerAddress, adminAddress, messengerAddress, nilGasPriceOracleAddress);
  }

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATING FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL1ETHBridge
  function depositETH(
    uint256 depositAmount,
    address payable l2DepositRecipient,
    address l2FeeRefundAddress,
    uint256 l2GasLimit,
    uint256 userMaxFeePerGas, // User-defined optional maxFeePerGas
    uint256 userMaxPriorityFeePerGas // User-defined optional maxPriorityFeePerGas
  ) external payable override whenNotPaused {
    DepositMessageParams memory depositMessageParams;
    depositMessageParams.depositorAddress = _msgSender();
    depositMessageParams.depositAmount = depositAmount;
    depositMessageParams.l2DepositRecipient = l2DepositRecipient;
    depositMessageParams.l2FeeRefundAddress = l2FeeRefundAddress;
    depositMessageParams.l2GasLimit = l2GasLimit;
    depositMessageParams.userMaxFeePerGas = userMaxFeePerGas;
    depositMessageParams.userMaxPriorityFeePerGas = userMaxPriorityFeePerGas;
    _deposit(depositMessageParams);
  }

  /// @inheritdoc IL1ETHBridge
  function depositETHViaRouter(
    address depositorAddress,
    uint256 depositAmount,
    address payable l2DepositRecipient,
    address l2FeeRefundAddress,
    uint256 l2GasLimit,
    uint256 userMaxFeePerGas, // User-defined optional maxFeePerGas
    uint256 userMaxPriorityFeePerGas // User-defined optional maxPriorityFeePerGas
  ) public payable override onlyRouter whenNotPaused {
    DepositMessageParams memory depositMessageParams;
    depositMessageParams.depositorAddress = depositorAddress;
    depositMessageParams.depositAmount = depositAmount;
    depositMessageParams.l2DepositRecipient = l2DepositRecipient;
    depositMessageParams.l2FeeRefundAddress = l2FeeRefundAddress;
    depositMessageParams.l2GasLimit = l2GasLimit;
    depositMessageParams.userMaxFeePerGas = userMaxFeePerGas;
    depositMessageParams.userMaxPriorityFeePerGas = userMaxPriorityFeePerGas;
    _deposit(depositMessageParams);
  }

  /// @inheritdoc IL1Bridge
  function cancelDeposit(bytes32 messageHash) public override nonReentrant {
    address caller = _msgSender();

    // get DepositMessageDetails
    L1BridgeMessengerEvents.DepositMessage memory depositMessage = IL1BridgeMessenger(messenger).getDepositMessage(
      messageHash
    );

    if (depositMessage.messageType != NilConstants.MessageType.DEPOSIT_ETH) {
      revert InvalidMessageType();
    }

    if (caller != router && caller != depositMessage.depositorAddress) {
      revert UnAuthorizedCaller();
    }

    // L1BridgeMessenger to verify if the deposit can be cancelled
    IL1BridgeMessenger(messenger).cancelDeposit(messageHash);

    // Refund the deposited ETH to the refundAddress
    (bool success, ) = payable(depositMessage.depositorAddress).call{ value: depositMessage.depositAmount }("");

    if (!success) {
      revert ErrorEthRefundFailed(messageHash);
    }

    emit DepositCancelled(messageHash, depositMessage.depositorAddress, depositMessage.depositAmount);
  }

  /// @inheritdoc IL1Bridge
  function claimFailedDeposit(
    bytes32 messageHash,
    bytes32[] memory claimProof
  ) public override nonReentrant whenNotPaused {
    L1BridgeMessengerEvents.DepositMessage memory depositMessage = IL1BridgeMessenger(messenger).getDepositMessage(
      messageHash
    );

    if (depositMessage.messageType != NilConstants.MessageType.DEPOSIT_ETH) {
      revert InvalidMessageType();
    }

    // L1BridgeMessenger to verify if the deposit can be claimed
    IL1BridgeMessenger(messenger).claimFailedDeposit(messageHash, claimProof);

    // Refund the deposited ETH to the refundAddress
    (bool success, ) = payable(depositMessage.depositorAddress).call{ value: depositMessage.depositAmount }("");

    if (!success) {
      revert ErrorEthRefundFailed(messageHash);
    }

    emit DepositClaimed(
      messageHash,
      depositMessage.tokenAddress,
      depositMessage.depositorAddress,
      depositMessage.depositAmount
    );
  }

  function finaliseETHWithdrawal(address l1WithdrawalRecipient, uint256 withdrawalAmount) public payable {}

  /*//////////////////////////////////////////////////////////////////////////
                             INTERNAL-FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @dev The internal ETH deposit implementation.
  /// @param _depositMessageParams The struct with parameters needed to build the DepositMessage and further processing via BridgeMessenger
  function _deposit(DepositMessageParams memory _depositMessageParams) internal virtual nonReentrant {
    if (_depositMessageParams.l2DepositRecipient == address(0)) {
      revert ErrorInvalidL2Recipient();
    }

    if (_depositMessageParams.depositAmount == 0) {
      revert ErrorZeroEthDeposit();
    }

    if (_depositMessageParams.l2GasLimit == 0) {
      revert ErrorInvalidL2GasLimit();
    }

    DepositETHMessageData memory depositETHMessageData;

    depositETHMessageData.feeCreditData = INilGasPriceOracle(nilGasPriceOracle).computeFeeCredit(
      _depositMessageParams.l2GasLimit,
      _depositMessageParams.userMaxFeePerGas,
      _depositMessageParams.userMaxPriorityFeePerGas
    );

    if (msg.value < _depositMessageParams.depositAmount + depositETHMessageData.feeCreditData.feeCredit) {
      revert ErrorInsufficientValueForFeeCredit();
    }

    depositETHMessageData.feeCreditData.nilGasLimit = _depositMessageParams.l2GasLimit;
    depositETHMessageData.depositorAddress = _depositMessageParams.depositorAddress;
    depositETHMessageData.depositAmount = _depositMessageParams.depositAmount;
    depositETHMessageData.l2DepositRecipient = payable(_depositMessageParams.l2DepositRecipient);
    depositETHMessageData.l2FeeRefundAddress = _depositMessageParams.l2FeeRefundAddress;

    // Generate message passed to L2ETHBridge
    depositETHMessageData.depositMessage = abi.encodeCall(
      IL2ETHBridge.finaliseETHDeposit,
      (
        depositETHMessageData.depositorAddress,
        depositETHMessageData.depositAmount,
        depositETHMessageData.l2DepositRecipient,
        depositETHMessageData.l2FeeRefundAddress
      )
    );

    // Send message to L1BridgeMessenger.
    IL1BridgeMessenger(messenger).sendMessage(
      NilConstants.MessageType.DEPOSIT_ETH,
      counterpartyBridge,
      depositETHMessageData.depositMessage,
      address(0),
      depositETHMessageData.depositorAddress,
      depositETHMessageData.depositAmount,
      depositETHMessageData.depositorAddress,
      depositETHMessageData.l2FeeRefundAddress,
      depositETHMessageData.feeCreditData
    );
  }

  /// @inheritdoc IERC165
  function supportsInterface(bytes4 interfaceId) public view override returns (bool) {
    return
      interfaceId == type(IL1ETHBridge).interfaceId ||
      interfaceId == type(IL1Bridge).interfaceId ||
      super.supportsInterface(interfaceId);
  }
}
