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
  mapping(address => address) public tokenMapping;

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

  function getL1ERC20Address(address l2Token) external view override returns (address) {
    return tokenMapping[l2Token];
  }

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATING FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  function finaliseERC20Deposit(
    address l1Token,
    address l2Token,
    address depositor,
    uint256 depositAmount,
    address depositRecipient,
    address feeRefundRecipient,
    bytes calldata additionalData
  ) external payable override onlyMessenger nonReentrant whenNotPaused {
    if (l1Token.isContract()) {
      revert ErrorInvalidL1TokenAddress();
    }

    // TODO - check if the l1TokenAddress is a contract address
    // TODO - check if the l2TokenAddress exists and is a contract
    // TODO - if the L1Token address mapping doesnot exist, it means the L2Token is to be created
    // TODO - Mapping for L1TokenAddress to be set

    if (l1Token != tokenMapping[l2Token]) {
      revert ErrorL1TokenAddressMismatch();
    }

    /// @notice Encoding the context to process the loan after the price is fetched
    /// @dev The context contains the borrower’s details, loan amount, borrow token, and collateral token.
    bytes memory tokenTransferContext = abi.encodeWithSelector(
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
    bytes memory mintCallData = abi.encodeWithSignature("mintTokenInternal(uint256)", depositAmount);

    /// @notice Send a request to the Oracle to get the price of the borrow token.
    /// @dev This request is processed with a fee for the transaction, allowing the system to fetch the token price.
    Nil.sendRequest(l2Token, 0, Nil.ASYNC_REQUEST_MIN_GAS, tokenTransferContext, mintCallData);
  }

  function handleTokenTransferResponse(bool success, bytes memory returnData, bytes memory context) public {
    /// @notice Ensure the Oracle call was successful
    /// @dev Verifies that the price data was successfully retrieved from the Oracle.
    require(success, "token call failed");

    /// @notice Decode the context to extract borrower details, loan amount, and collateral token
    /// @dev Decodes the context passed from the borrow function to retrieve necessary data.
    (
      TokenId l1Token,
      TokenId l2Token,
      address depositor,
      uint256 depositAmount,
      address depositRecipient,
      address feeRefundRecipient,
      bytes memory additionalData
    ) = abi.decode(context, (TokenId, TokenId, address, uint256, address, address, bytes));

    NilTokenBase.sendTokenInternal(depositRecipient, l2Token, depositAmount);

    emit FinalizeDepositERC20(
      l1Token,
      l2Token,
      depositor,
      depositAmount,
      depositRecipient,
      feeRefundRecipient,
      additionalData
    );
  }

  /*//////////////////////////////////////////////////////////////////////////
                         RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2EnshrinedTokenBridge
  function setTokenMapping(address l2EnshrinedTokenAddress, address l1TokenAddress) external override onlyOwnerOrAdmin {
    if (!l2EnshrinedTokenAddress.isContract() || !l1TokenAddress.isContract()) {
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
