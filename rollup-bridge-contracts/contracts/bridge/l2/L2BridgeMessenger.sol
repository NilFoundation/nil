// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";
import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { AccessControlEnumerableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/extensions/AccessControlEnumerableUpgradeable.sol";
import { NilConstants } from "../../common/libraries/NilConstants.sol";
import { IRelayMessage } from "./interfaces/IRelayMessage.sol";
import { ErrorInvalidMessageType } from "../../common/NilErrorConstants.sol";
import { AddressChecker } from "../../common/libraries/AddressChecker.sol";
import { StorageUtils } from "../../common/libraries/StorageUtils.sol";
import { IL2BridgeMessenger } from "./interfaces/IL2BridgeMessenger.sol";
import { IBridgeMessenger } from "../interfaces/IBridgeMessenger.sol";
import { IL2Bridge } from "./interfaces/IL2Bridge.sol";
import { IBridge } from "../interfaces/IBridge.sol";
import { INilMessageTree } from "../../interfaces/INilMessageTree.sol";

/// @title L2BridgeMessenger
/// @notice The `L2BridgeMessenger` contract can:
/// 1. send messages from nil-chain to layer 1
/// 2. receive relayed messages from L1 via relayer
/// 3. entrypoint for all messages relayed from layer-1 to nil-chain via relayer
contract L2BridgeMessenger is
  OwnableUpgradeable,
  PausableUpgradeable,
  ReentrancyGuardUpgradeable,
  AccessControlEnumerableUpgradeable,
  IL2BridgeMessenger
{
  using EnumerableSet for EnumerableSet.AddressSet;
  using EnumerableSet for EnumerableSet.Bytes32Set;
  using AddressChecker for address;
  using StorageUtils for bytes32;

  /*//////////////////////////////////////////////////////////////////////////
                                  STATE VARIABLES
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice address of the bridgeMessenger from counterpart (L1) chain
  address public override counterpartyBridgeMessenger;

  address public override nilMessageTree;

  uint256 public override messageExpiryDelta;

  /// @notice  Holds the addresses of authorised bridges that can interact to send messages.
  EnumerableSet.AddressSet private authorisedBridges;

  /// @notice EnumerableSet for messageHash of the message relayed by relayer on behalf of L1BridgeMessenger
  EnumerableSet.Bytes32Set private relayedMessageHashStore;

  /// @notice EnumerableSet for messageHash of the withdrawal-messages sent from L2BridgeMessenger for further relay
  /// to L1 via Relayer
  EnumerableSet.Bytes32Set private withdrawalMessageHashStore;

  /// @notice The nonce for withdraw messages.
  uint256 public override withdrawalNonce;

  /// @notice the aggregated hash for all message-hash values received by the l2BridgeMessenger
  /// @dev initialize with the genesis state Hash during the contract initialisation
  bytes32 public override l1MessageHash;

  /// @dev The storage slots for future usage.
  uint256[50] private __gap;

  /*//////////////////////////////////////////////////////////////////////////
                                    CONSTRUCTOR
    //////////////////////////////////////////////////////////////////////////*/

  constructor() {
    _disableInitializers();
  }

  /*//////////////////////////////////////////////////////////////////////////
                                    INITIALIZER
    //////////////////////////////////////////////////////////////////////////*/

  function initialize(
    address ownerAddress,
    address adminAddress,
    address relayerAddress,
    address nilMessageTreeAddress,
    uint256 messageExpiryDeltaValue
  ) public initializer {
    // Validate input parameters
    if (ownerAddress == address(0)) {
      revert ErrorInvalidOwner();
    }

    if (adminAddress == address(0)) {
      revert ErrorInvalidDefaultAdmin();
    }

    if (nilMessageTreeAddress == address(0)) {
      revert ErrorInvalidAddress();
    }

    // Initialize the Ownable contract with the owner address
    OwnableUpgradeable.__Ownable_init(ownerAddress);

    // Initialize the Pausable contract
    PausableUpgradeable.__Pausable_init();

    // Initialize the AccessControlEnumerable contract
    __AccessControlEnumerable_init();

    ReentrancyGuardUpgradeable.__ReentrancyGuard_init();

    // Set role admins
    // The OWNER_ROLE is set as its own admin to ensure that only the current owner can manage this role.
    _setRoleAdmin(NilConstants.OWNER_ROLE, NilConstants.OWNER_ROLE);

    // The DEFAULT_ADMIN_ROLE is set as its own admin to ensure that only the current default admin can manage this
    // role.
    _setRoleAdmin(DEFAULT_ADMIN_ROLE, NilConstants.OWNER_ROLE);

    // Grant roles to defaultAdmin and owner
    // The DEFAULT_ADMIN_ROLE is granted to both the default admin and the owner to ensure that both have the
    // highest level of control.
    // The OWNER_ROLE is granted to the owner to ensure they have the highest level of control over the contract.
    _grantRole(NilConstants.OWNER_ROLE, ownerAddress);
    _grantRole(DEFAULT_ADMIN_ROLE, adminAddress);

    _grantRole(NilConstants.RELAYER_ROLE_ADMIN, adminAddress);
    _grantRole(NilConstants.RELAYER_ROLE_ADMIN, ownerAddress);
    _grantRole(NilConstants.RELAYER_ROLE, ownerAddress);
    _grantRole(NilConstants.RELAYER_ROLE, adminAddress);

    if (relayerAddress.isContract()) {
      _grantRole(NilConstants.RELAYER_ROLE, relayerAddress);
    }

    nilMessageTree = nilMessageTreeAddress;
    messageExpiryDelta = messageExpiryDeltaValue;
  }

  // make sure only owner can send ether to messenger to avoid possible user fund loss.
  receive() external payable onlyOwner {}

  /*//////////////////////////////////////////////////////////////////////////
                             MODIFIERS
    //////////////////////////////////////////////////////////////////////////*/

  modifier onlyauthorisedL2Bridge() {
    if (!authorisedBridges.contains(msg.sender)) {
      revert ErrorBridgeNotAuthorised();
    }
    _;
  }

  modifier onlyRelayer() {
    if (!hasRole(NilConstants.RELAYER_ROLE, msg.sender)) {
      revert ErrorRelayerNotAuthorised();
    }
    _;
  }

  modifier onlyAdmin() {
    if (!(hasRole(DEFAULT_ADMIN_ROLE, msg.sender))) {
      revert ErrorCallerIsNotAdmin();
    }
    _;
  }

  modifier onlyOwnerOrAdmin() {
    if (!(hasRole(DEFAULT_ADMIN_ROLE, msg.sender)) && !(hasRole(NilConstants.OWNER_ROLE, msg.sender))) {
      revert ErrorCallerNotAuthorised();
    }
    _;
  }

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS  
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2BridgeMessenger
  function getAuthorisedBridges() public view override returns (address[] memory) {
    return authorisedBridges.values();
  }

  /// @inheritdoc IL2BridgeMessenger
  function isAuthorisedBridge(address bridgeAddress) public view override returns (bool) {
    return authorisedBridges.contains(bridgeAddress);
  }

  /// @inheritdoc IL2BridgeMessenger
  function isFullyInitialised() public view returns (bool) {
    address[] memory relayers = getRoleMembers(NilConstants.RELAYER_ROLE);
    address[] memory authorisedBridgeAddreses = getAuthorisedBridges();

    if (!counterpartyBridgeMessenger.isContract() || relayers.length == 0 || authorisedBridgeAddreses.length == 0) {
      return false;
    }

    return true;
  }

  modifier onlyAuthorizedL2Bridge() {
    if (!authorisedBridges.contains(msg.sender)) {
      revert BridgeNotAuthorized();
    }
    _;
  }

  /*//////////////////////////////////////////////////////////////////////////
                         PUBLIC MUTATION FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2BridgeMessenger
  function sendMessage(
    NilConstants.MessageType messageType,
    address messageTarget,
    bytes memory message
  ) public payable override whenNotPaused onlyAuthorizedL2Bridge returns (bytes32) {
    return
      _sendMessage(SendMessageParams({ messageType: messageType, messageTarget: messageTarget, message: message }));
  }

  /// @inheritdoc IRelayMessage
  function relayMessage(
    address messageSender,
    address messageTarget,
    NilConstants.MessageType messageType,
    uint256 messageNonce,
    bytes memory message,
    uint256 messageExpiryTime
  ) external override onlyRelayer whenNotPaused {
    if (messageType != NilConstants.MessageType.DEPOSIT_ERC20 && messageType != NilConstants.MessageType.DEPOSIT_ETH) {
      revert ErrorInvalidMessageType();
    }

    bytes32 messageHash = computeDepositMessageHash(messageType, messageSender, messageTarget, messageNonce, message);

    if (relayedMessageHashStore.contains(messageHash)) {
      revert ErrorDuplicateMessageRelayed(messageHash);
    }

    relayedMessageHashStore.add(messageHash);

    if (l1MessageHash == bytes32(0)) {
      l1MessageHash = messageHash;
    } else {
      l1MessageHash = keccak256(abi.encode(messageHash, l1MessageHash));
    }

    if (messageExpiryTime < block.timestamp + messageExpiryDelta) {
      INilMessageTree(nilMessageTree).appendMessage(messageHash);
      emit MessageExecutionFailed(messageHash);
    } else {
      bool isExecutionSuccessful = _executeMessage(messageTarget, message);

      if (!isExecutionSuccessful) {
        INilMessageTree(nilMessageTree).appendMessage(messageHash);
        emit MessageExecutionFailed(messageHash);
      } else {
        emit MessageExecutionSuccessful(messageHash);
      }
    }
  }

  /// @inheritdoc IL2BridgeMessenger
  function computeDepositMessageHash(
    NilConstants.MessageType messageType,
    address messageSender,
    address messageTarget,
    uint256 messageNonce,
    bytes memory message
  ) public pure override returns (bytes32) {
    return keccak256(abi.encode(messageType, messageSender, messageTarget, messageNonce, message));
  }

  /// @inheritdoc IL2BridgeMessenger
  function computeWithdrawalMessageHash(
    NilConstants.MessageType messageType,
    address messageSender,
    address messageTarget,
    uint256 messageNonce,
    bytes memory message
  ) public pure override returns (bytes32) {
    return keccak256(abi.encode(messageType, messageSender, messageTarget, messageNonce, message));
  }

  /*//////////////////////////////////////////////////////////////////////////
                         INTERNAL FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  function _executeMessage(address _messageTarget, bytes memory _message) internal returns (bool) {
    // @note check `_messageTarget` address to avoid attack in the future when we add more gateways.
    if (!isAuthorisedBridge(_messageTarget)) {
      revert ErrorBridgeNotAuthorised();
    }
    (bool isSuccessful, ) = (_messageTarget).call(_message);
    return isSuccessful;
  }

  /*//////////////////////////////////////////////////////////////////////////
                             INTERNAL FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  function _sendMessage(SendMessageParams memory params) internal nonReentrant returns (bytes32) {
    bytes32 messageHash = computeWithdrawalMessageHash(
      params.messageType,
      _msgSender(),
      params.messageTarget,
      withdrawalNonce,
      params.message
    );

    if (withdrawalMessageHashStore.contains(messageHash)) {
      revert ErrorDuplicateWithdrawalMessage(messageHash);
    }

    withdrawalMessageHashStore.add(messageHash);

    // append messageHash as leaf to the NilMessageTree
    // all withdrawalMessageHashes must be appended to the merkleTree
    (uint256 merkleTreeLeafIndex, bytes32 merkleRoot) = INilMessageTree(nilMessageTree).appendMessage(messageHash);

    emit MessageSent(
      _msgSender(),
      params.messageTarget,
      withdrawalNonce,
      merkleTreeLeafIndex,
      params.message,
      messageHash,
      params.messageType,
      block.timestamp
    );

    withdrawalNonce = withdrawalNonce + 1;

    return messageHash;
  }

  /*//////////////////////////////////////////////////////////////////////////
                         RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2BridgeMessenger
  function setCounterpartyBridgeMessenger(
    address counterpartyBridgeMessengerAddress
  ) external override onlyOwnerOrAdmin {
    _setCounterpartyBridgeMessenger(counterpartyBridgeMessengerAddress);
  }

  function _setCounterpartyBridgeMessenger(address counterpartyBridgeMessengerAddress) internal {
    if (!counterpartyBridgeMessengerAddress.isContract()) {
      revert ErrorInvalidBridgeMessenger();
    }
    emit CounterpartyBridgeMessengerSet(counterpartyBridgeMessenger, counterpartyBridgeMessengerAddress);
    counterpartyBridgeMessenger = counterpartyBridgeMessengerAddress;
  }

  /// @inheritdoc IL2BridgeMessenger
  function authoriseBridges(address[] calldata bridges) external override onlyOwnerOrAdmin {
    for (uint256 i = 0; i < bridges.length; i++) {
      _authoriseBridge(bridges[i]);
    }
  }

  /// @inheritdoc IL2BridgeMessenger
  function authoriseBridge(address bridge) external override onlyOwnerOrAdmin {
    _authoriseBridge(bridge);
  }

  function _authoriseBridge(address bridge) internal {
    if (!IERC165(IBridge(bridge).getImplementation()).supportsInterface(type(IL2Bridge).interfaceId)) {
      revert ErrorInvalidBridgeInterface();
    }
    if (authorisedBridges.contains(bridge)) {
      revert ErrorBridgeAlreadyAuthorised();
    }
    authorisedBridges.add(bridge);
  }

  /// @inheritdoc IL2BridgeMessenger
  function revokeBridgeAuthorisation(address bridge) external override onlyOwnerOrAdmin {
    if (!authorisedBridges.contains(bridge)) {
      revert ErrorBridgeNotAuthorised();
    }
    authorisedBridges.remove(bridge);
  }

  function grantRelayerRole(address relayerAddress) external override onlyOwnerOrAdmin {
    _grantRole(NilConstants.RELAYER_ROLE, relayerAddress);
  }

  /// @inheritdoc IL2BridgeMessenger
  function setPause(bool _status) external override onlyOwnerOrAdmin {
    if (_status) {
      _pause();
    } else {
      _unpause();
    }
  }

  /// @inheritdoc IBridgeMessenger
  function transferOwnershipRole(address newOwner) external override onlyOwner {
    _revokeRole(NilConstants.OWNER_ROLE, owner());
    super.transferOwnership(newOwner);
    _grantRole(NilConstants.OWNER_ROLE, newOwner);
  }

  /**
   * @dev Returns the current implementation address.
   */
  function getImplementation() public view override returns (address) {
    return StorageUtils.getImplementationAddress(NilConstants.IMPLEMENTATION_SLOT);
  }

  /// @inheritdoc IERC165
  function supportsInterface(
    bytes4 interfaceId
  ) public view override(AccessControlEnumerableUpgradeable, IERC165) returns (bool) {
    return interfaceId == type(IL2BridgeMessenger).interfaceId || super.supportsInterface(interfaceId);
  }

  /// @inheritdoc IL2BridgeMessenger
  function hasRelayerRole(address relayerAddress) external view override returns (bool) {
    return hasRole(NilConstants.RELAYER_ROLE, relayerAddress);
  }

  function getL2ToL1Root() external view override returns (bytes32) {
    return INilMessageTree(nilMessageTree).getMessageRoot();
  }

  /// @inheritdoc IL2BridgeMessenger
  function getLatestDepositNonce() external view returns (uint256) {
    return relayedMessageHashStore.length();
  }
}
