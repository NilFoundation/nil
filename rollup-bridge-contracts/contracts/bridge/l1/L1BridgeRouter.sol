// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { NilAccessControlUpgradeable } from "../../NilAccessControlUpgradeable.sol";
import { NilConstants } from "../../common/libraries/NilConstants.sol";
import { ERC20 } from "../../common/tokens/ERC20.sol";
import { SafeTransferLib } from "../../common/libraries/SafeTransferLib.sol";
import { AddressChecker } from "../../common/libraries/AddressChecker.sol";
import { StorageUtils } from "../../common/libraries/StorageUtils.sol";
import { IL1ERC20Bridge } from "./interfaces/IL1ERC20Bridge.sol";
import { IL1ETHBridge } from "./interfaces/IL1ETHBridge.sol";
import { IBridgeMessenger } from "../interfaces/IBridgeMessenger.sol";
import { IBridge } from "../interfaces/IBridge.sol";
import { IL1BridgeRouter } from "./interfaces/IL1BridgeRouter.sol";
import { IL1BridgeMessenger } from "./interfaces/IL1BridgeMessenger.sol";

/// @title L1BridgeRouter
/// @notice The `L1BridgeRouter` is the main entry for depositing ERC20 tokens.
/// All deposited tokens are routed to corresponding gateways.
/// @dev use this contract to query L1/L2 token address mapping.
contract L1BridgeRouter is
    OwnableUpgradeable,
    PausableUpgradeable,
    NilAccessControlUpgradeable,
    ReentrancyGuardUpgradeable,
    IL1BridgeRouter
{
    using SafeTransferLib for ERC20;
    using AddressChecker for address;
    using StorageUtils for bytes32;

    /*//////////////////////////////////////////////////////////////////////////
                             STATE-VARIABLES   
    //////////////////////////////////////////////////////////////////////////*/

    /// @notice The addess of ERC20Bridge
    address public override erc20Bridge;

    /// @notice The address of ETHBridge
    address public override ethBridge;

    /// @notice The addess of BridgeMessenger on L1
    IL1BridgeMessenger public messenger;

    /// @notice The address of the WETH token
    address public override wethAddress;

    /// @notice The address of l1Bridge in current execution context.
    address transient public l1BridgeInContext;

    /*//////////////////////////////////////////////////////////////////////////
                             FUNCTION-MODIFIERS   
    //////////////////////////////////////////////////////////////////////////*/

    modifier onlyNotInContext() {
        require(l1BridgeInContext == address(0), "Only not in context");
        _;
    }

    modifier onlyInContext() {
        require(_msgSender() == l1BridgeInContext, "Only in deposit context");
        _;
    }

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

    /// @notice Initialize the storage of L1BridgeRouter.
    /// @param ownerAddress The address of owner for L1BridgeRouter contract.
    /// @param adminAddress The address of admin for L1BridgeRouter contract.
    /// @param erc20BridgeAddress The address of L1ERC20Bridge contract.
    /// @param ethBridgeAddress The address of L1ETHBridge contract.
    /// @param messengerAddress The address of L1BridgeMessenger contract.
    /// @param weth The address of weth token on L1.
    function initialize(
        address ownerAddress,
        address adminAddress,
        address erc20BridgeAddress,
        address ethBridgeAddress,
        address messengerAddress,
        address weth
    )
        public
        initializer
    {
        // Validate input parameters
        if (ownerAddress == address(0)) {
            revert ErrorInvalidOwner();
        }

        if (adminAddress == address(0)) {
            revert ErrorInvalidDefaultAdmin();
        }

        // Initialize the Ownable contract with the owner address
        OwnableUpgradeable.__Ownable_init(ownerAddress);

        // Initialize the Pausable contract
        PausableUpgradeable.__Pausable_init();

        // Initialize the AccessControlEnumerable contract
        __AccessControlEnumerable_init();

        // Set role admins
        // The OWNER_ROLE is set as its own admin to ensure that only the current owner can manage this role.
        _setRoleAdmin(NilConstants.OWNER_ROLE, NilConstants.OWNER_ROLE);

        // The DEFAULT_ADMIN_ROLE is set as its own admin to ensure that only the current default admin can manage this
        // role.
        _setRoleAdmin(DEFAULT_ADMIN_ROLE, NilConstants.OWNER_ROLE);

        // Grant roles to defaultAdmin and owner
        // The DEFAULT_ADMIN_ROLE is granted to both the default admin and the owner to ensure that both have the
        // highest level of control.
        // The PROPOSER_ROLE_ADMIN is granted to both the default admin and the owner to allow them to manage proposers.
        // The OWNER_ROLE is granted to the owner to ensure they have the highest level of control over the contract.
        _grantRole(NilConstants.OWNER_ROLE, ownerAddress);
        _grantRole(DEFAULT_ADMIN_ROLE, adminAddress);

        ReentrancyGuardUpgradeable.__ReentrancyGuard_init();

        _setERC20Bridge(erc20BridgeAddress);
        _setETHBridge(ethBridgeAddress);
        _setMessenger(messengerAddress);
        _setWETHAddress(weth);
    }

    /*//////////////////////////////////////////////////////////////////////////
                           USER-FACING CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc IL1BridgeRouter
    function getL2TokenAddress(address _l1TokenAddress) external view override returns (address) {
        return IL1ERC20Bridge(erc20Bridge).getL2TokenAddress(_l1TokenAddress);
    }

    /**
    * @dev Returns the current implementation address.
    */
    function getImplementation() public view override returns (address) {
        return StorageUtils.getImplementationAddress(NilConstants.IMPLEMENTATION_SLOT);
    }

    /*//////////////////////////////////////////////////////////////////////////
                           BRIDGE-SPECIFIC MUTATION FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc IL1BridgeRouter
    function pullERC20(address sender, address token, uint256 amount) external onlyInContext returns (uint256) {
        address _bridgeAddress = _msgSender();

        // Validate that the caller is one of the authorized bridges
        if (_bridgeAddress != address(erc20Bridge)) {
            revert ErrorUnauthorizedCaller();
        }

        // Get the current balance of the bridge contract
        uint256 _balance = ERC20(token).balanceOf(_bridgeAddress);

        // Transfer tokens from the sender to the bridge contract
        ERC20(token).safeTransferFrom(sender, _bridgeAddress, amount);

        // Calculate the actual amount of tokens pulled
        uint256 _amountPulled = ERC20(token).balanceOf(_bridgeAddress) - _balance;

        // Ensure the pulled amount matches the requested amount
        if (_amountPulled != amount) {
            revert ErrorERC20PullFailed();
        }

        return _amountPulled;
    }

    /*//////////////////////////////////////////////////////////////////////////
                           USER-SPECIFIC MUTATION FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc IL1BridgeRouter
    function depositERC20(
        address l1Token,
        uint256 depositAmount,
        address l2DepositRecipient,
        address l2FeeRefundAddress,
        uint256 l2GasLimit,
        uint256 userMaxFeePerGas, // User-defined optional maxFeePerGas
        uint256 userMaxPriorityFeePerGas // User-defined optional maxPriorityFeePerGas
    )
        public
        payable
        override
        onlyNotInContext
        whenNotPaused
    {
        if (l1Token == address(0)) {
            revert ErrorInvalidTokenAddress();
        }

        if (l1Token == wethAddress) {
            revert ErrorWETHTokenNotSupported();
        }

        if (erc20Bridge == address(0)) {
            revert ErrorInvalidL1ERC20BridgeAddress();
        }

        if (depositAmount == 0) {
            revert ErrorEmptyDeposit();
        }

        if (l2GasLimit == 0) {
            revert ErrorInvalidNilGasLimit();
        }

        // enter deposit context
        l1BridgeInContext = erc20Bridge;

        IL1ERC20Bridge(erc20Bridge).depositERC20ViaRouter{ value: msg.value }(
            l1Token,
            _msgSender(),
            depositAmount,
            l2DepositRecipient,
            l2FeeRefundAddress,
            l2GasLimit,
            userMaxFeePerGas,
            userMaxPriorityFeePerGas
        );

        // leave deposit context
        l1BridgeInContext = address(0);
    }

    /// @inheritdoc IL1BridgeRouter
    function depositETH(
        uint256 depositAmount,
        address payable l2DepositRecipient,
        address l2FeeRefundAddress,
        uint256 l2GasLimit,
        uint256 userMaxFeePerGas,
        uint256 userMaxPriorityFeePerGas
    )
        public
        payable
        override
        onlyNotInContext
        whenNotPaused
    {
        if (ethBridge == address(0)) {
            revert ErrorInvalidL1ETHBridgeAddress();
        }

        if (l2DepositRecipient == address(0)) {
            revert ErrorInvalidL2DepositRecipient();
        }

        if (depositAmount == 0) {
            revert ErrorEmptyDeposit();
        }

        if (l2GasLimit == 0) {
            revert ErrorInvalidNilGasLimit();
        }

        // enter deposit context
        l1BridgeInContext = ethBridge;

        IL1ETHBridge(ethBridge).depositETHViaRouter{ value: msg.value }(
            _msgSender(),
            depositAmount,
            l2DepositRecipient,
            l2FeeRefundAddress,
            l2GasLimit,
            userMaxFeePerGas,
            userMaxPriorityFeePerGas
        );

        // leave deposit context
        l1BridgeInContext = address(0);
    }

    /// @inheritdoc IL1BridgeRouter
    function cancelDeposit(bytes32 messageHash) external payable whenNotPaused {
        // Get the deposit message from the messenger
        NilConstants.MessageType messageType = messenger.getMessageType(messageHash);

        // Route the cancellation request based on the deposit type
        if (messageType == NilConstants.MessageType.DEPOSIT_ERC20) {
            IL1ERC20Bridge(erc20Bridge).cancelDeposit(messageHash);
        } else if (messageType == NilConstants.MessageType.DEPOSIT_ETH) {
            IL1ETHBridge(ethBridge).cancelDeposit(messageHash);
        } else {
            revert ErrorInvalidMessageType();
        }
    }

    /// @inheritdoc IL1BridgeRouter
    function claimFailedDeposit(bytes32 messageHash, uint256 merkleTreeLeafIndex, bytes32[] memory claimProof) external override whenNotPaused {
        // Get the deposit message from the messenger
        NilConstants.MessageType messageType = messenger.getMessageType(messageHash);

        // Route the cancellation request based on the deposit type
        if (messageType == NilConstants.MessageType.DEPOSIT_ERC20) {
            IL1ERC20Bridge(erc20Bridge).claimFailedDeposit(messageHash, merkleTreeLeafIndex, claimProof);
        } else if (messageType == NilConstants.MessageType.DEPOSIT_ETH) {
            IL1ETHBridge(ethBridge).claimFailedDeposit(messageHash, merkleTreeLeafIndex, claimProof);
        } else {
            revert ErrorInvalidMessageType();
        }
    }

    /*//////////////////////////////////////////////////////////////////////////
                           RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc IL1BridgeRouter
    function setERC20Bridge(address erc20BridgeAddress) external override onlyOwnerOrAdmin {
        _setERC20Bridge(erc20BridgeAddress);
    }

    function _setERC20Bridge(address _erc20BridgeAddress) internal {

        if (
            !_erc20BridgeAddress.isContract()
                || !IERC165(IBridge(_erc20BridgeAddress).getImplementation()).supportsInterface(type(IL1ERC20Bridge).interfaceId)
        ) {
            revert ErrorInvalidERC20Bridge();
        }
        address oldERC20Bridge = erc20Bridge;
        erc20Bridge = _erc20BridgeAddress;
        emit ERC20BridgeSet(oldERC20Bridge, _erc20BridgeAddress);
    }

    /// @inheritdoc IL1BridgeRouter
    function setETHBridge(address ethBridgeAddress) external override onlyOwnerOrAdmin {
        _setETHBridge(ethBridgeAddress);
    }

    function _setETHBridge(address _ethBridgeAddress) internal {

        if (
            !_ethBridgeAddress.isContract()
                || !IERC165(IBridge(_ethBridgeAddress).getImplementation()).supportsInterface(type(IL1ETHBridge).interfaceId)
        ) {
            revert ErrorInvalidL1ETHBridgeAddress();
        }
        address oldETHBridge = ethBridge;
        ethBridge = _ethBridgeAddress;
        emit ETHBridgeSet(oldETHBridge, _ethBridgeAddress);
    }

    /// @inheritdoc IL1BridgeRouter
    function setWETHAddress(address weth) external override onlyOwnerOrAdmin {
        _setWETHAddress(weth);
    }

    function _setWETHAddress(address _weth) internal {

        if (!_weth.isContract()) {
            revert ErrorInvalidTokenAddress();
        }

        address oldWETHAddress = wethAddress;
        wethAddress = _weth;
        emit WETHSet(oldWETHAddress, _weth);
    }

    /// @inheritdoc IL1BridgeRouter
    function setMessenger(address messengerAddress) external override onlyOwnerOrAdmin {
        _setMessenger(messengerAddress);
    }

    function _setMessenger(address _messengerAddress) internal {
        if (
            !_messengerAddress.isContract()
                || !IERC165(IBridgeMessenger(_messengerAddress).getImplementation()).supportsInterface(type(IL1BridgeMessenger).interfaceId)
        ) {
            revert ErrorInvalidMessenger();
        }

        address oldMessenger = _messengerAddress;
        messenger = IL1BridgeMessenger(_messengerAddress);
        emit MessengerSet(oldMessenger, _messengerAddress);
    }

    /// @inheritdoc IL1BridgeRouter
    function setPause(bool _status) external onlyOwnerOrAdmin {
        if (_status) {
            _pause();
        } else {
            _unpause();
        }
    }

    /// @inheritdoc IL1BridgeRouter
    function transferOwnershipRole(address newOwner) external override onlyOwner {
        _revokeRole(NilConstants.OWNER_ROLE, owner());
        super.transferOwnership(newOwner);
        _grantRole(NilConstants.OWNER_ROLE, newOwner);
    }

    /// @inheritdoc IERC165
  function supportsInterface(bytes4 interfaceId) public view override returns (bool) {
    return
      interfaceId == type(IL1BridgeRouter).interfaceId ||
      super.supportsInterface(interfaceId);
  }
}
