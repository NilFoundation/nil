// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

interface INilETHBridge {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS   
    //////////////////////////////////////////////////////////////////////////*/

  error ErrorCallerIsNotAdmin();

  error ErrorCallerNotAuthorised();

  error ErrorCallerIsNotMessenger();

  error ErrorInvalidOwner();

  error ErrorInvalidDefaultAdmin();

  /*//////////////////////////////////////////////////////////////////////////
                             EVENTS   
    //////////////////////////////////////////////////////////////////////////*/

  event FinaliseETHDeposit(address indexed from, address to, uint256 amount);

  event L2ETHBridgeVaultSet(address indexed l2ETHBridgeVault, address indexed newL2ETHBridgeVault);

  event WithdrawnETH(bytes32 indexed messageHash, address l1WithdrawalRecipient, uint256 withdrawalAmount);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC RESTRICTED FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  function l2ETHBridgeVault() external returns (address);

  /*//////////////////////////////////////////////////////////////////////////
                            PUBLIC MUTATION FUNCTIONS      
    //////////////////////////////////////////////////////////////////////////*/

  // function getImplementation() external view returns (address);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice The address of Bridge contract on other side (for L1Bridge it would be the bridge-address on L2 and for
  /// L2Bridge this would be the bridge-address on L1)
  function counterpartyBridge() external view returns (address);

  /// @notice The address of corresponding L1NilMessenger/L2NilMessenger contract.
  function messenger() external view returns (address);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  // function grantAccess(bytes32 role, address account) external;

  // function revokeAccess(bytes32 role, address account) external;
}
