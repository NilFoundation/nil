// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { NilAccessControlUpgradeable } from "../../NilAccessControlUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { AccessControlEnumerableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/extensions/AccessControlEnumerableUpgradeable.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { AddressChecker } from "../../common/libraries/AddressChecker.sol";
import { NilConstants } from "../../common/libraries/NilConstants.sol";
import { IL1ERC20Bridge } from "../l1/interfaces/IL1ERC20Bridge.sol";
import { IL2EnshrinedTokenBridge } from "./interfaces/IL2EnshrinedTokenBridge.sol";
import { IL2Bridge } from "./interfaces/IL2Bridge.sol";
import { IL2BridgeMessenger } from "./interfaces/IL2BridgeMessenger.sol";
import { IL2BridgeRouter } from "./interfaces/IL2BridgeRouter.sol";
import { L2BaseBridge } from "./L2BaseBridge.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract L2EnshrinedTokenBridge is L2BaseBridge, IL2EnshrinedTokenBridge, NilBase, NilTokenBase {
  using AddressChecker for address;

  /// @notice Mapping from enshrined-token-address to layer-1 ERC20-TokenAddress.
  // solhint-disable-next-line var-name-mixedcase
  mapping(TokenId => address) public tokenMapping;

  /// @dev The storage slots for future usage.
  uint256[50] private __gap;

  /*//////////////////////////////////////////////////////////////////////////
                                    CONSTRUCTOR
    //////////////////////////////////////////////////////////////////////////*/

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /*//////////////////////////////////////////////////////////////////////////
                                    INITIALIZER
    //////////////////////////////////////////////////////////////////////////*/

  function initialize(address ownerAddress, address adminAddress, address messengerAddress) public initializer {
    // Validate input parameters
    if (ownerAddress == address(0)) {
      revert ErrorInvalidOwner();
    }

    if (adminAddress == address(0)) {
      revert ErrorInvalidDefaultAdmin();
    }

    L2BaseBridge.__L2BaseBridge_init(ownerAddress, adminAddress, messengerAddress);
  }

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  function getL1ERC20Address(TokenId l2Token) external view override returns (address) {
    return tokenMapping[l2Token];
  }

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATING FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /**
   * @notice Finalizes an ERC20 deposit from L1 to L2.
   * @dev Mints tokens to the current contract and initiates an async request to transfer them to the deposit recipient.
   * @param l1Token The address of the ERC20 token on L1.
   * @param l2Token The address of the corresponding ERC20 token on L2.
   * @param depositor The address of the depositor on L1.
   * @param depositAmount The amount of tokens deposited.
   * @param depositRecipient The address of the recipient on L2.
   * @param feeRefundRecipient The address to refund any excess fees on L2.
   * @param additionalData Additional data for processing the deposit.
   */
  function finaliseERC20Deposit(
    address l1Token,
    TokenId l2Token,
    address depositor,
    uint256 depositAmount,
    address depositRecipient,
    address feeRefundRecipient,
    bytes calldata additionalData
  ) external payable override onlyMessenger nonReentrant whenNotPaused {
    if (l1Token.isContract()) {
      revert ErrorInvalidL1TokenAddress();
    }

    if (tokenMapping[l2Token] != address(0) && l1Token != tokenMapping[l2Token]) {
      revert ErrorL1TokenAddressMismatch();
    }

    if (tokenMapping[l2Token] == address(0)) {
      // TODO - check if the l1TokenAddress is a contract address
      // TODO - check if the l2TokenAddress exists and is a contract
      // TODO - if the L1Token address mapping doesnot exist, it means the L2Token is to be created
      // TODO - Mapping for L1TokenAddress to be set
    }

    /// @notice Prepare a call to the token contract to mint the tokens
    bytes memory mintCallData = abi.encodeWithSignature("mintTokenInternal(uint256)", depositAmount);
    Nil.Token[] memory emptyTokens;
    (bool success, bytes memory result) = Nil.syncCall(
      TokenId.unwrap(l2Token),
      gasleft(),
      0,
      emptyTokens,
      mintCallData
    );

    if (!success) {
      revert ErrorMintTokenFailed();
    }

    //NilTokenBase.sendTokenInternal(depositRecipient, l2Token, depositAmount);

    emit FinalizedDepositERC20(
      l1Token,
      l2Token,
      depositor,
      depositAmount,
      depositRecipient,
      feeRefundRecipient,
      additionalData
    );
  }

  function withdrawEnshrinedToken(address l1WithdrawRecipient, uint256 withdrawalAmount) public {
    // validate for l1WithdrawalRecipient
    if (!l1WithdrawRecipient.isContract()) {
      revert ErrorInvalidAddress();
    }

    // validate the withdrawalAmount
    if (withdrawalAmount == 0) {
      revert ErrorInvalidAmount();
    }

    /// @notice Retrieve the tokens being sent in the transaction
    Nil.Token[] memory tokens = Nil.txnTokens();

    // check if the l1Token exists for the TokenId in NilToken being withdrawn
    if (tokenMapping[tokens[0].id] == address(0)) {
      revert ErrorNoL1TokenMapping();
    }

    /// @dev declare an empty list of tokens (to be sent on to destination address)
    Nil.Token[] memory tokenList;

    Nil.syncCall(
      TokenId.unwrap(tokens[0].id), // destination address
      gasleft(), //gas
      0, // value
      tokenList, // empty token List
      abi.encodeWithSignature("burnTokenInternal(uint256)", withdrawalAmount) // calldata
    );

    // Generate message to be executed on L1ETHBridge
    bytes memory message = abi.encodeCall(
      IL1ERC20Bridge.finaliseWithdrawERC20,
      (TokenId.unwrap(tokens[0].id), tokenMapping[tokens[0].id], _msgSender(), l1WithdrawRecipient, withdrawalAmount)
    );

    // Send message to L2BridgeMessenger.
    bytes32 messageHash = IL2BridgeMessenger(messenger).sendMessage(
      NilConstants.MessageType.WITHDRAW_ENSHRINED_TOKEN,
      counterpartyBridge,
      message
    );
  }

  /*//////////////////////////////////////////////////////////////////////////
                         RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2EnshrinedTokenBridge
  function setTokenMapping(TokenId l2EnshrinedTokenAddress, address l1TokenAddress) external override onlyOwnerOrAdmin {
    if (!l1TokenAddress.isContract()) {
      revert ErrorInvalidTokenAddress();
    }

    // TODO - check if the tokenAddresses are not EOA and a valid contract
    // TODO - check if the l2EnshrinedTokenAddress implement ERC-165 or any common interface

    tokenMapping[l2EnshrinedTokenAddress] = l1TokenAddress;

    emit TokenMappingUpdated(l2EnshrinedTokenAddress, l1TokenAddress);
  }

  /// @inheritdoc IERC165
  function supportsInterface(bytes4 interfaceId) public view override returns (bool) {
    return
      interfaceId == type(IL2EnshrinedTokenBridge).interfaceId ||
      interfaceId == type(IL2Bridge).interfaceId ||
      super.supportsInterface(interfaceId);
  }
}
