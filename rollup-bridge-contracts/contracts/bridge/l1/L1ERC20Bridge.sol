// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { ERC20 } from "solmate/tokens/ERC20.sol";
import { SafeTransferLib } from "solmate/utils/SafeTransferLib.sol";
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
  bytes4 public constant FINALIZE_ERC20_DEPOSIT_SELECTOR = IL2EnshrinedTokenBridge.finalizeERC20Deposit.selector;

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
    address token,
    address l2DepositRecipient,
    uint256 depositAmount,
    address l2FeeRefundRecipient,
    uint256 l2GasLimit,
    uint256 userFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) external payable override whenNotPaused {
    _deposit(
      token,
      l2DepositRecipient,
      depositAmount,
      l2FeeRefundRecipient,
      _msgSender(),
      l2GasLimit,
      userFeePerGas,
      userMaxPriorityFeePerGas
    );
  }

  function depositERC20ViaRouter(
    address token,
    address l2DepositRecipient,
    uint256 depositAmount,
    address l2FeeRefundRecipient,
    address depositorAddress,
    uint256 l2GasLimit,
    uint256 userFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) public payable override onlyRouter whenNotPaused {
    _deposit(
      token,
      l2DepositRecipient,
      depositAmount,
      l2FeeRefundRecipient,
      depositorAddress,
      l2GasLimit,
      userFeePerGas,
      userMaxPriorityFeePerGas
    );
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

    // Decode the message to extract the token address and the original sender (_from)
    ERC20DepositMessage memory erc20DecodedDepositMessage = decodeERC20DepositMessage(depositMessage.message);

    if (depositMessage.messageType != NilConstants.MessageType.DEPOSIT_ERC20) {
      revert InvalidMessageType();
    }

    if (caller != router && caller != erc20DecodedDepositMessage.depositorAddress) {
      revert UnAuthorizedCaller();
    }

    // L1BridgeMessenger to verify if the deposit can be cancelled
    IL1BridgeMessenger(messenger).cancelDeposit(messageHash);

    // refund the deposited ERC20 tokens to the refundAddress
    ERC20(erc20DecodedDepositMessage.l1Token).safeTransfer(
      depositMessage.l1DepositRefundAddress,
      erc20DecodedDepositMessage.depositAmount
    );

    emit DepositCancelled(
      messageHash,
      erc20DecodedDepositMessage.l1Token,
      depositMessage.l1DepositRefundAddress,
      erc20DecodedDepositMessage.depositAmount
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

    // Decode the message to extract the token address and the original sender (_from)
    ERC20DepositMessage memory erc20DepositMessage = decodeERC20DepositMessage(depositMessage.message);

    if (depositMessage.messageType != NilConstants.MessageType.DEPOSIT_ERC20) {
      revert InvalidMessageType();
    }

    // L1BridgeMessenger to verify if the deposit can be claimed
    IL1BridgeMessenger(messenger).claimFailedDeposit(messageHash, claimProof);

    // refund the deposit-amount
    ERC20(erc20DepositMessage.l1Token).safeTransfer(
      depositMessage.l1DepositRefundAddress,
      erc20DepositMessage.depositAmount
    );

    emit DepositClaimed(
      messageHash,
      erc20DepositMessage.l1Token,
      depositMessage.l1DepositRefundAddress,
      erc20DepositMessage.depositAmount
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
  /// @param _l1Token The token to deposit.
  /// @param _l2DepositRecipient The recipient address to recieve the token in L2.
  /// @param _depositAmount The amount of token to deposit.
  /// @param _l2FeeRefundRecipient the address of recipient for excess fee refund.
  /// @param _depositorAddress the address of depositor
  /// @param _nilGasLimit Gas limit required to complete the deposit on L2.
  /// @param _userMaxFeePerGas The maximum Fee per gas unit that the user is willing to pay.
  /// @param _userMaxPriorityFeePerGas The maximum priority fee per gas unit that the user is willing to pay.
  function _deposit(
    address _l1Token,
    address _l2DepositRecipient,
    uint256 _depositAmount,
    address _l2FeeRefundRecipient,
    address _depositorAddress,
    uint256 _nilGasLimit,
    uint256 _userMaxFeePerGas,
    uint256 _userMaxPriorityFeePerGas
  ) internal virtual nonReentrant {
    if (_l1Token == address(0)) {
      revert ErrorInvalidTokenAddress();
    }

    if (_l1Token == wethToken) {
      revert ErrorWETHTokenNotSupported();
    }

    if (_l2DepositRecipient == address(0)) {
      revert ErrorInvalidL2DepositRecipient();
    }

    if (_depositAmount == 0) {
      revert ErrorEmptyDeposit();
    }

    if (_l2FeeRefundRecipient == address(0)) {
      revert ErrorInvalidL2FeeRefundRecipient();
    }

    if (_nilGasLimit == 0) {
      revert ErrorInvalidNilGasLimit();
    }

    address _l2Token = tokenMapping[_l1Token];

    if (_l2Token.isContract()) {
      //TODO compute l2TokenAddress
      //shardId, bytecode, salt, , ... -> l2TokenAddress
      // update the mapping
    }

    if (_l2Token == address(0)) {
      revert ErrorInvalidL2Token();
    }

    // Transfer token into Bridge contract
    _transferERC20In(_l1Token, _depositAmount, _depositorAddress);

    INilGasPriceOracle.FeeCreditData memory feeCreditData = INilGasPriceOracle(nilGasPriceOracle).computeFeeCredit(
      _nilGasLimit,
      _userMaxFeePerGas,
      _userMaxPriorityFeePerGas
    );

    if (msg.value < feeCreditData.feeCredit) {
      revert ErrorInsufficientValueForFeeCredit();
    }

    feeCreditData.nilGasLimit = _nilGasLimit;

    // should we refund excess msg.value back to user?
    // is the fees locked is refunded during

    // TODO encode token symbol, token decimals
    // TODO encoded token-metadata is needed only for the token which doesn't exist in the mapping
    bytes memory _data;

    // Generate message passed to L2ERC20Bridge
    bytes memory _message = abi.encodeCall(
      IL2EnshrinedTokenBridge.finalizeERC20Deposit,
      (_l1Token, _l2Token, _depositorAddress, _l2DepositRecipient, _l2FeeRefundRecipient, _depositAmount, _data)
    );

    // Send message to L1BridgeMessenger.
    IL1BridgeMessenger(messenger).sendMessage{ value: msg.value }(
      NilConstants.MessageType.DEPOSIT_ETH,
      counterpartyBridge,
      0,
      _message,
      _depositorAddress,
      feeCreditData
    );

    emit DepositERC20(_l1Token, _l2Token, _depositorAddress, _l2DepositRecipient, _depositAmount, _data);
  }

  /// @inheritdoc IL1ERC20Bridge
  function decodeERC20DepositMessage(bytes memory _message) public pure returns (ERC20DepositMessage memory) {
    // Validate that the first 4 bytes of the message match the function selector
    bytes4 selector;
    assembly {
      selector := mload(add(_message, 32))
    }
    if (selector != FINALIZE_ERC20_DEPOSIT_SELECTOR) {
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

    (
      address l1Token,
      address l2Token,
      address depositorAddress,
      address l2DepositRecipient,
      address l2FeeRefundRecipient,
      uint256 depositAmount,
      bytes memory data
    ) = abi.decode(messageData, (address, address, address, address, address, uint256, bytes));

    return
      ERC20DepositMessage({
        l1Token: l1Token,
        l2Token: l2Token,
        depositorAddress: depositorAddress,
        l2DepositRecipient: l2DepositRecipient,
        l2FeeRefundRecipient: l2FeeRefundRecipient,
        depositAmount: depositAmount,
        additionalData: data
      });
  }

  /// @inheritdoc IERC165
  function supportsInterface(bytes4 interfaceId) public view override returns (bool) {
    return
      interfaceId == type(IL1ERC20Bridge).interfaceId ||
      interfaceId == type(IL1Bridge).interfaceId ||
      super.supportsInterface(interfaceId);
  }
}
