// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { ERC20 } from "../../common/tokens/ERC20.sol";
import { SafeTransferLib } from "../../common/libraries/SafeTransferLib.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { NilConstants } from "../../common/libraries/NilConstants.sol";
import { AddressChecker } from "../../common/libraries/AddressChecker.sol";
import { NilAccessControlUpgradeable } from "../../NilAccessControlUpgradeable.sol";
import { IL1ERC20Bridge } from "./interfaces/IL1ERC20Bridge.sol";
import { IL2EnshrinedTokenBridge } from "../l2/interfaces/IL2EnshrinedTokenBridge.sol";
import { IL1BridgeRouter } from "./interfaces/IL1BridgeRouter.sol";
import { IL1Bridge } from "./interfaces/IL1Bridge.sol";
import { IBridge } from "../interfaces/IBridge.sol";
import { IL1BridgeMessenger } from "./interfaces/IL1BridgeMessenger.sol";
import { INilGasPriceOracle } from "./interfaces/INilGasPriceOracle.sol";
import { L1BaseBridge } from "./L1BaseBridge.sol";

/// @title L1ERC20Bridge
/// @notice The `L1ERC20Bridge` contract for ERC20Bridging in L1.
contract L1ERC20Bridge is L1BaseBridge, IL1ERC20Bridge {
  using SafeTransferLib for ERC20;
  using AddressChecker for address;

  // Define the function selector for finalizeDepositERC20 as a constant
  bytes4 public constant FINALIZE_ERC20_DEPOSIT_SELECTOR =
    bytes4(keccak256("finaliseERC20Deposit(address,address,address,uint256,address,address,bytes)"));

  /*//////////////////////////////////////////////////////////////////////////
                             STATE-VARIABLES   
    //////////////////////////////////////////////////////////////////////////*/

  address public override wethToken;

  /// @notice Mapping from l1 token address to l2 token address for ERC20 token.
  mapping(address => address) public tokenMapping;

  /// @dev The storage slots for future usage.
  uint256[50] private __gap;

  /*//////////////////////////////////////////////////////////////////////////
                             CONSTRUCTOR   
    //////////////////////////////////////////////////////////////////////////*/

  /// @custom:oz-upgrades-unsafe-allow constructor
  /// @notice Constructor for `L1ERC20Bridge` implementation contract.
  constructor() {
    _disableInitializers();
  }

  /// @notice Initialize the storage of L1ERC20Bridge.
  /// @param ownerAddress The owner of L1ERC20Bridge
  /// @param adminAddress The address of admin who is granted DEFAULT_ADMIN role on L1ERC20Bridge.
  /// @param wethTokenAddress The address of WETH token on L1
  /// @param messengerAddress The address of L1BridgeMessengewethTokenAddress
  /// @param nilGasPriceOracleAddress The address of NilGasPriceOracle on L1
  function initialize(
    address ownerAddress,
    address adminAddress,
    address wethTokenAddress,
    address messengerAddress,
    address nilGasPriceOracleAddress
  ) public initializer {
    if (!wethTokenAddress.isContract()) {
      revert ErrorInvalidWethToken();
    }

    L1BaseBridge.__L1BaseBridge_init(ownerAddress, adminAddress, messengerAddress, nilGasPriceOracleAddress);

    wethToken = wethTokenAddress;
  }

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATING FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL1ERC20Bridge
  function depositERC20(
    address l1Token,
    uint256 depositAmount,
    address l2DepositRecipient,
    address l2FeeRefundAddress,
    uint256 l2GasLimit,
    uint256 userMaxFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) public payable override whenNotPaused {
    DepositMessageParams memory depositMessageParams;
    depositMessageParams.l1Token = l1Token;
    depositMessageParams.depositorAddress = _msgSender();
    depositMessageParams.depositAmount = depositAmount;
    depositMessageParams.l2DepositRecipient = l2DepositRecipient;
    depositMessageParams.l2FeeRefundAddress = l2FeeRefundAddress;
    depositMessageParams.l2GasLimit = l2GasLimit;
    depositMessageParams.userMaxFeePerGas = userMaxFeePerGas;
    depositMessageParams.userMaxPriorityFeePerGas = userMaxPriorityFeePerGas;
    _deposit(depositMessageParams);
  }

  function depositERC20ViaRouter(
    address l1Token,
    address depositorAddress,
    uint256 depositAmount,
    address l2DepositRecipient,
    address l2FeeRefundAddress,
    uint256 l2GasLimit,
    uint256 userMaxFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) public payable override onlyRouter whenNotPaused {
    DepositMessageParams memory depositMessageParams;
    depositMessageParams.l1Token = l1Token;
    depositMessageParams.depositorAddress = depositorAddress;
    depositMessageParams.depositAmount = depositAmount;
    depositMessageParams.l2DepositRecipient = l2DepositRecipient;
    depositMessageParams.l2FeeRefundAddress = l2FeeRefundAddress;
    depositMessageParams.l2GasLimit = l2GasLimit;
    depositMessageParams.userMaxFeePerGas = userMaxFeePerGas;
    depositMessageParams.userMaxPriorityFeePerGas = userMaxPriorityFeePerGas;
    _deposit(depositMessageParams);
  }

  /// @inheritdoc IL1ERC20Bridge
  function getL2TokenAddress(address _l1TokenAddress) external view override returns (address) {
    return tokenMapping[_l1TokenAddress];
  }

  /// @inheritdoc IL1Bridge
  function cancelDeposit(bytes32 messageHash) public override nonReentrant whenNotPaused {
    address caller = _msgSender();

    // get DepositMessageDetails
    IL1BridgeMessenger.DepositMessage memory depositMessage = IL1BridgeMessenger(messenger).getDepositMessage(
      messageHash
    );

    if (depositMessage.messageType != NilConstants.MessageType.DEPOSIT_ERC20) {
      revert InvalidMessageType();
    }

    if (caller != router && caller != depositMessage.depositorAddress) {
      revert UnAuthorizedCaller();
    }

    // L1BridgeMessenger to verify if the deposit can be cancelled
    IL1BridgeMessenger(messenger).cancelDeposit(messageHash);

    // refund the deposited ERC20 tokens to the refundAddress
    ERC20(depositMessage.tokenAddress).safeTransfer(depositMessage.depositorAddress, depositMessage.depositAmount);

    emit DepositCancelled(
      messageHash,
      depositMessage.tokenAddress,
      depositMessage.depositorAddress,
      depositMessage.depositAmount
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

    if (depositMessage.messageType != NilConstants.MessageType.DEPOSIT_ERC20) {
      revert InvalidMessageType();
    }

    // L1BridgeMessenger to verify if the deposit can be claimed
    IL1BridgeMessenger(messenger).claimFailedDeposit(messageHash, claimProof);

    // refund the deposit-amount
    ERC20(depositMessage.tokenAddress).safeTransfer(depositMessage.depositorAddress, depositMessage.depositAmount);

    emit DepositClaimed(
      messageHash,
      depositMessage.tokenAddress,
      depositMessage.depositorAddress,
      depositMessage.depositAmount
    );
  }

  /*//////////////////////////////////////////////////////////////////////////
                         RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL1ERC20Bridge
  function setTokenMapping(address l1TokenAddress, address l2EnshrinedTokenAddress) external override onlyOwnerOrAdmin {
    if (!l2EnshrinedTokenAddress.isContract() || !l1TokenAddress.isContract()) {
      revert ErrorInvalidTokenAddress();
    }
    address oldL2EnshrinedTokenAddress = tokenMapping[l1TokenAddress];
    tokenMapping[l1TokenAddress] = l2EnshrinedTokenAddress;
    emit UpdatedTokenMapping(l1TokenAddress, oldL2EnshrinedTokenAddress, l2EnshrinedTokenAddress);
  }

  /*//////////////////////////////////////////////////////////////////////////
                             INTERNAL-FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @dev Internal function to transfer ERC20 token to this contract.
  /// @param _l1Token The address of token to transfer.
  /// @param _depositAmount The amount of token to transfer.
  /// @param _depositorAddress The address of depositor who initiated the deposit transaction.
  /// @dev If the depositor called depositERC20 via L1BridgeRouter, then _sender will be the l1BridgeRouter-address
  /// If the depositor called depositERC20 directly on L1ERC20Bridge, then _sender will be the
  /// l1ERC20Bridge-address
  function _transferERC20In(address _l1Token, uint256 _depositAmount, address _depositorAddress) internal {
    uint256 _amountPulled = 0;

    if (router == _msgSender()) {
      // _depositor will be derived from the routerData as the depositor called on router directly
      // _sender will be router-address and its router's responsibility to pull the ERC20Token from depositor to
      // L1ERC20Bridge
      _amountPulled = IL1BridgeRouter(router).pullERC20(_depositorAddress, _l1Token, _depositAmount);
    } else {
      uint256 _tokenBalanceBeforePull = ERC20(_l1Token).balanceOf(address(this));

      // L1ERC20Bridge to transfer ERC20 Tokens from depositor address to the L1ERC20Bridge
      // L1ERC20Bridge must have sufficient approval of spending on ERC20Token
      ERC20(_l1Token).safeTransferFrom(_depositorAddress, address(this), _depositAmount);

      _amountPulled = ERC20(_l1Token).balanceOf(address(this)) - _tokenBalanceBeforePull;
    }

    if (_amountPulled != _depositAmount) {
      revert ErrorIncorrectAmountPulledByBridge();
    }
  }

  /// @dev Internal function to do all the deposit operations.
  /// @param _depositMessageParams The struct with parameters needed to build the DepositMessage and further processing via BridgeMessenger
  function _deposit(DepositMessageParams memory _depositMessageParams) internal virtual nonReentrant {
    if (_depositMessageParams.l1Token == address(0)) {
      revert ErrorInvalidTokenAddress();
    }

    if (_depositMessageParams.l1Token == wethToken) {
      revert ErrorWETHTokenNotSupported();
    }

    if (_depositMessageParams.l2DepositRecipient == address(0)) {
      revert ErrorInvalidL2DepositRecipient();
    }

    if (_depositMessageParams.depositAmount == 0) {
      revert ErrorEmptyDeposit();
    }

    if (_depositMessageParams.l2GasLimit == 0) {
      revert ErrorInvalidNilGasLimit();
    }

    _depositMessageParams.l2Token = tokenMapping[_depositMessageParams.l1Token];

    if (_depositMessageParams.l2Token.isContract()) {
      //TODO compute l2TokenAddress
      //shardId, bytecode, salt, , ... -> l2TokenAddress
      // update the mapping
    }

    if (_depositMessageParams.l2Token == address(0)) {
      revert ErrorInvalidL2Token();
    }

    // Transfer token into Bridge contract
    _transferERC20In(
      _depositMessageParams.l1Token,
      _depositMessageParams.depositAmount,
      _depositMessageParams.depositorAddress
    );

    _depositMessageParams.feeCreditData = INilGasPriceOracle(nilGasPriceOracle).computeFeeCredit(
      _depositMessageParams.l2GasLimit,
      _depositMessageParams.userMaxFeePerGas,
      _depositMessageParams.userMaxPriorityFeePerGas
    );

    if (msg.value < _depositMessageParams.feeCreditData.feeCredit) {
      revert ErrorInsufficientValueForFeeCredit();
    }

    _depositMessageParams.feeCreditData.nilGasLimit = _depositMessageParams.l2GasLimit;

    // should we refund excess msg.value back to user?
    // is the fees locked is refunded during

    // TODO encode token symbol, token decimals
    // TODO encoded token-metadata is needed only for the token which doesn't exist in the mapping

    // Prepare data for sendMessage
    DepositERC20MessageData memory _depositERC20MessageData = DepositERC20MessageData({
      l1Token: _depositMessageParams.l1Token,
      l2Token: _depositMessageParams.l2Token,
      depositorAddress: _depositMessageParams.depositorAddress,
      depositAmount: _depositMessageParams.depositAmount,
      l2DepositRecipient: _depositMessageParams.l2DepositRecipient,
      l2FeeRefundAddress: _depositMessageParams.l2FeeRefundAddress,
      data: _depositMessageParams.additionalData,
      feeCreditData: _depositMessageParams.feeCreditData
    });

    // Generate message passed to L2ERC20Bridge
    _depositMessageParams.message = abi.encodeCall(
      IL2EnshrinedTokenBridge.finaliseERC20Deposit,
      (
        _depositMessageParams.l1Token,
        _depositMessageParams.l2Token,
        _depositMessageParams.depositorAddress,
        _depositMessageParams.depositAmount,
        _depositMessageParams.l2DepositRecipient,
        _depositMessageParams.l2FeeRefundAddress,
        _depositMessageParams.additionalData
      )
    );

    // Send message to L1BridgeMessenger
    IL1BridgeMessenger(messenger).sendMessage(
      NilConstants.MessageType.DEPOSIT_ETH,
      counterpartyBridge,
      _depositMessageParams.message,
      _depositERC20MessageData.l1Token,
      _depositERC20MessageData.depositorAddress,
      _depositERC20MessageData.depositAmount,
      _depositERC20MessageData.depositorAddress,
      _depositERC20MessageData.l2FeeRefundAddress,
      _depositERC20MessageData.feeCreditData
    );
  }

  /// @inheritdoc IERC165
  function supportsInterface(bytes4 interfaceId) public view override returns (bool) {
    return
      interfaceId == type(IL1ERC20Bridge).interfaceId ||
      interfaceId == type(IL1Bridge).interfaceId ||
      super.supportsInterface(interfaceId);
  }
}
