// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import "@nilfoundation/smart-contracts/contracts/Nil.sol";
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
import { NilEnshrinedToken } from "../../common/tokens/NilEnshrinedToken.sol";

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
   * @dev Mints tokens to the current contract and initiates an async request to transfer them to the deposit
   * recipient.
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

    address l1TokenFromMapping = tokenMapping[l2Token];

    if (l1TokenFromMapping != address(0) && l1Token != l1TokenFromMapping) {
      revert ErrorL1TokenAddressMismatch();
    }

    if (l1TokenFromMapping == address(0)) {
      // L1Token address mapping doesnot exist and L2Token is to be created (Factory to create the token)
      // decode additionalData to get the inputs for Factory call
      string memory tokenName = abi.decode(additionalData, (string));

      bytes memory l2TokenCreationBytes = abi.encodePacked(type(NilEnshrinedToken).creationCode, abi.encode(tokenName));

      uint256 salt = uint256(uint160(l1Token));

      address l2EnshrinedTokenCreated = Nil.asyncDeploy(1, _msgSender(), 0, l2TokenCreationBytes, salt);

      if (
        l2EnshrinedTokenCreated.code.length == 0 ||
        l2EnshrinedTokenCreated == address(0) ||
        TokenId.wrap(l2EnshrinedTokenCreated) != l2Token
      ) {
        revert ErrorL2TokenCreationFailed();
      }

      // Mapping for L1TokenAddress to be set
      tokenMapping[l2Token] = l1Token;
    }

    TokenId l1TokenId = TokenId.wrap(l1Token);
    address l2TokenAddress = TokenId.unwrap(l2Token);

    /// @notice Prepare a call to the token contract to mint the tokens
    bytes memory mintCallData = abi.encodeWithSignature("mintTokenInternal(uint256)", depositAmount);
    Nil.Token[] memory emptyTokens;
    (bool success, bytes memory result) = Nil.syncCall(l2TokenAddress, gasleft(), 0, emptyTokens, mintCallData);

    if (!success) {
      revert ErrorMintTokenFailed();
    }

    /// @notice Send the mined tokens from L2EnshrinedTokenBridge to the depositRecipient
    /// @dev Transfers the token amount to the depositRecipient's address after the mint is successful.
    /// @notice Encoding the context to process the loan after the price is fetched
    /// @dev The context contains the borrower’s details, loan amount, borrow token, and collateral token.
    bytes memory eventEmissionContext = abi.encodeWithSelector(
      this.handleTokenTransferResponse.selector,
      l1Token,
      l2Token,
      depositor,
      depositAmount,
      depositRecipient,
      feeRefundRecipient,
      additionalData
    );

    /// @notice Prepare a call to the token contract to mint the tokens
    bytes memory transferCallData = abi.encodeWithSignature(
      "sendTokenInternal(address,address,uint256)",
      depositRecipient,
      l2Token,
      depositAmount
    );

    /// @notice Send a request to the token contract to get token minted.
    /// @dev This request is processed with a fee for the transaction, allowing the system to fetch the token price.
    Nil.sendRequest(l2TokenAddress, 0, Nil.ASYNC_REQUEST_MIN_GAS, eventEmissionContext, transferCallData);
  }

  function handleTokenTransferResponse(bool success, bytes memory returnData, bytes memory context) public {
    /// @notice Ensure the token-mint call was successful
    /// @dev Verifies that the enshrined-token was minted successfully.
    if (!success) {
      revert ErrorTokenTransferFailed();
    }

    /// @notice Decode the context to extract deposit details and feeRefundRecipient
    /// @dev Decodes the context passed from the handleTokenMintResponse function to retrieve necessary data.
    (
      TokenId l1Token,
      TokenId l2Token,
      address depositor,
      uint256 depositAmount,
      address depositRecipient,
      address feeRefundRecipient,
      bytes memory additionalData
    ) = abi.decode(context, (TokenId, TokenId, address, uint256, address, address, bytes));

    emit FinalisedDepositERC20(
      TokenId.unwrap(l1Token),
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

    if (tokens.length != 1) {
      revert ErrorInvalidTokenCount();
    }

    address l1TokenAddress = tokenMapping[tokens[0].id];
    address l2TokenAddress = TokenId.unwrap(tokens[0].id);

    // check if the l1Token exists for the TokenId in NilToken being withdrawn
    if (l1TokenAddress == address(0)) {
      revert ErrorNoL1TokenMapping();
    }

    /// @notice Encoding the context to process the callback for burnTokenInternal's completion
    bytes memory postTokenBurnCallbackContext = abi.encodeWithSelector(
      this.handleTokenBurnResponse.selector,
      l1TokenAddress,
      l2TokenAddress,
      _msgSender(),
      l1WithdrawRecipient,
      withdrawalAmount
    );

    /// @notice Prepare a call to the token contract to burn the tokens
    bytes memory burnInternalCallData = abi.encodeWithSignature("burnTokenInternal(uint256)", withdrawalAmount);

    /// @notice Send a request to the token contract to get token burnt.
    Nil.sendRequest(l2TokenAddress, 0, Nil.ASYNC_REQUEST_MIN_GAS, postTokenBurnCallbackContext, burnInternalCallData);
  }

  function handleTokenBurnResponse(bool success, bytes memory returnData, bytes memory context) public {
    /// @notice Ensure the Token burn call was successful
    if (!success) {
      revert ErrorTokenBurnFailed();
    }

    /// @notice Decode the context to extract deposit details and feeRefundRecipient
    /// @dev Decodes the context passed from the handleTokenMintResponse function to retrieve necessary data.
    (
      address l1Token,
      address l2Token,
      address withdrawerAddress,
      address withdrawalRecipient,
      uint256 withdrawalAmount
    ) = abi.decode(context, (address, address, address, address, uint256));

    // Generate message to be executed on L1ETHBridge
    bytes memory message = abi.encodeCall(
      IL1ERC20Bridge.finaliseWithdrawERC20,
      (l1Token, l2Token, withdrawerAddress, withdrawalRecipient, withdrawalAmount)
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
