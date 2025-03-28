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
    address to,
    uint256 amount,
    address l2FeeRefundRecipient,
    uint256 gasLimit,
    uint256 userFeePerGas, // User-defined optional maxFeePerGas
    uint256 userMaxPriorityFeePerGas // User-defined optional maxPriorityFeePerGas
  ) external payable override whenNotPaused {
    _deposit(to, amount, l2FeeRefundRecipient, _msgSender(), gasLimit, userFeePerGas, userMaxPriorityFeePerGas);
  }

  /// @inheritdoc IL1ETHBridge
  function depositETHViaRouter(
    address to,
    uint256 amount,
    address l2FeeRefundRecipient,
    address depositorAddress,
    uint256 gasLimit,
    uint256 userFeePerGas, // User-defined optional maxFeePerGas
    uint256 userMaxPriorityFeePerGas // User-defined optional maxPriorityFeePerGas
  ) public payable override onlyRouter whenNotPaused {
    _deposit(to, amount, l2FeeRefundRecipient, depositorAddress, gasLimit, userFeePerGas, userMaxPriorityFeePerGas);
  }

  /// @inheritdoc IL1Bridge
  function cancelDeposit(bytes32 messageHash) public override nonReentrant {
    address caller = _msgSender();

    // get DepositMessageDetails
    IL1BridgeMessenger.DepositMessage memory depositMessage = IL1BridgeMessenger(messenger).getDepositMessage(
      messageHash
    );

    if (depositMessage.messageType != NilConstants.MessageType.DEPOSIT_ETH) {
      revert InvalidMessageType();
    }

    ETHDecodedDepositMessage memory ethDecodedDepositMessage = decodeETHDepositMessage(depositMessage.message);

    if (caller != router && caller != ethDecodedDepositMessage.depositorAddress) {
      revert UnAuthorizedCaller();
    }

    // L1BridgeMessenger to verify if the deposit can be cancelled
    IL1BridgeMessenger(messenger).cancelDeposit(messageHash);

    // Refund the deposited ETH to the refundAddress
    (bool success, ) = payable(depositMessage.l1DepositRefundAddress).call{
      value: ethDecodedDepositMessage.depositAmount
    }("");

    if (!success) {
      revert ErrorEthRefundFailed(messageHash);
    }

    emit DepositCancelled(
      messageHash,
      ethDecodedDepositMessage.depositorAddress,
      ethDecodedDepositMessage.depositAmount
    );
  }

  /// @inheritdoc IL1Bridge
  function claimFailedDeposit(
    bytes32 messageHash,
    bytes32[] memory claimProof
  ) public override nonReentrant whenNotPaused {
    IL1BridgeMessenger.DepositMessage memory depositMessage = IL1BridgeMessenger(messenger).getDepositMessage(
      messageHash
    );

    ETHDecodedDepositMessage memory ethDecodedDepositMessage = decodeETHDepositMessage(depositMessage.message);

    if (depositMessage.messageType != NilConstants.MessageType.DEPOSIT_ETH) {
      revert InvalidMessageType();
    }

    // L1BridgeMessenger to verify if the deposit can be claimed
    IL1BridgeMessenger(messenger).claimFailedDeposit(messageHash, claimProof);

    // Refund the deposited ETH to the refundAddress
    (bool success, ) = payable(depositMessage.l1DepositRefundAddress).call{
      value: ethDecodedDepositMessage.depositAmount
    }("");

    if (!success) {
      revert ErrorEthRefundFailed(messageHash);
    }

    emit DepositClaimed(
      messageHash,
      address(0),
      depositMessage.l1DepositRefundAddress,
      ethDecodedDepositMessage.depositAmount
    );
  }

  /*//////////////////////////////////////////////////////////////////////////
                             INTERNAL-FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @dev The internal ETH deposit implementation.
  /// @param _l2DepositRecipient The recipient address on nil-chain.
  /// @param _depositAmount The amount of ETH to be deposited.
  /// @param _l2FeeRefundRecipient The address of recipient to receive the excess-fee refund on nil-chain
  /// @param _depositorAddress The address of depositor
  /// @param _nilGasLimit Gas limit required to complete the deposit on nil-chain.
  /// @param _userMaxFeePerGas The maximum Fee per gas unit that the user is willing to pay.
  /// @param _userMaxPriorityFeePerGas The maximum priority fee per gas unit that the user is willing to pay.
  function _deposit(
    address _l2DepositRecipient,
    uint256 _depositAmount,
    address _l2FeeRefundRecipient,
    address _depositorAddress,
    uint256 _nilGasLimit,
    uint256 _userMaxFeePerGas, // User-defined optional maxFeePerGas
    uint256 _userMaxPriorityFeePerGas // User-defined optional maxPriorityFeePerGas
  ) internal virtual nonReentrant {
    if (_l2DepositRecipient == address(0)) {
      revert ErrorInvalidL2Recipient();
    }

    if (_depositAmount == 0) {
      revert ErrorZeroEthDeposit();
    }

    if (_l2FeeRefundRecipient == address(0)) {
      revert ErrorInvalidL2FeeRefundRecipient();
    }

    if (_nilGasLimit == 0) {
      revert ErrorInvalidL2GasLimit();
    }

    INilGasPriceOracle.FeeCreditData memory feeCreditData = INilGasPriceOracle(nilGasPriceOracle).computeFeeCredit(
      _nilGasLimit,
      _userMaxFeePerGas,
      _userMaxPriorityFeePerGas
    );

    if (msg.value < _depositAmount + feeCreditData.feeCredit) {
      revert ErrorInsufficientValueForFeeCredit();
    }

    feeCreditData.nilGasLimit = _nilGasLimit;

    // Generate message passed to L2ERC20Bridge
    bytes memory _message = abi.encodeCall(
      IL2ETHBridge.finaliseETHDeposit,
      (_depositorAddress, payable(_l2DepositRecipient), _l2FeeRefundRecipient, _depositAmount)
    );

    // Send message to L1BridgeMessenger.
    IL1BridgeMessenger(messenger).sendMessage{ value: msg.value }(
      NilConstants.MessageType.DEPOSIT_ETH,
      counterpartyBridge,
      _depositAmount,
      _message,
      _depositorAddress,
      feeCreditData
    );

    emit DepositETH(_depositorAddress, _l2DepositRecipient, _depositAmount);
  }

  /// @inheritdoc IL1ETHBridge
  function decodeETHDepositMessage(
    bytes memory _message
  ) public pure override returns (ETHDecodedDepositMessage memory) {
    // Validate that the first 4 bytes of the message match the function selector
    bytes4 selector;
    assembly {
      selector := mload(add(_message, 32))
    }
    if (selector != FINALISE_DEPOSIT_ETH_SELECTOR) {
      revert ErrorInvalidFinaliseDepositFunctionSelector();
    }

    // Extract the data part of the message
    bytes memory messageData;
    assembly {
      let dataLength := sub(mload(_message), 4)
      messageData := mload(0x40)
      mstore(messageData, dataLength)
      mstore(0x40, add(messageData, add(dataLength, 32)))
      mstore(add(messageData, 32), mload(add(_message, 36)))
    }

    (address depositorAddress, address l2DepositRecipient, address l2FeeRefundRecipient, uint256 depositAmount) = abi
      .decode(messageData, (address, address, address, uint256));

    return
      ETHDecodedDepositMessage({
        depositorAddress: depositorAddress,
        l2DepositRecipient: l2DepositRecipient,
        l2FeeRefundRecipient: l2FeeRefundRecipient,
        depositAmount: depositAmount
      });
  }

  /// @inheritdoc IERC165
  function supportsInterface(bytes4 interfaceId) public view override returns (bool) {
    return
      interfaceId == type(IL1ETHBridge).interfaceId ||
      interfaceId == type(IL1Bridge).interfaceId ||
      super.supportsInterface(interfaceId);
  }
}
