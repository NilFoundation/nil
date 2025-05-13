// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import { Pausable } from "@openzeppelin/contracts/utils/Pausable.sol";
import { AppendOnlyMerkleTree } from "./AppendOnlyMerkleTree.sol";
import { INilMessageTree } from "../interfaces/INilMessageTree.sol";

/**
 * @title NilMessageTree
 * @notice A contract for maintaining an append-only Merkle tree for messages.
 * @dev This contract inherits from `AppendOnlyMerkleTree` and provides functionality
 *      to append messages to the Merkle tree and compute the updated Merkle root.
 *      It is designed to work with the `L2BridgeMessenger` contract and ensures
 *      that only the authorized messenger can append messages.
 */
contract NilMessageTree is AppendOnlyMerkleTree, Ownable, Initializable, INilMessageTree {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS
    //////////////////////////////////////////////////////////////////////////*/

  error ErrorUnauthorised();

  /*//////////////////////////////////////////////////////////////////////////
                             STATE-VARIABLES   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice The address of L2BridgeMessenger contract.
  address public messenger;

  /*//////////////////////////////////////////////////////////////////////////
                             CONSTRUCTOR  
    //////////////////////////////////////////////////////////////////////////*/

  constructor(address _owner) Ownable(_owner) {}

  /*//////////////////////////////////////////////////////////////////////////
                             INITIALIZER  
    //////////////////////////////////////////////////////////////////////////*/

  /**
   * @notice Initializes the Merkle tree.
   * @dev This function can only be called once, enforced by the `initializer` modifier.
   * @custom:throws "cannot initialize" if the tree has already been initialized with messages.
   */
  function initialize() external initializer {
    _initializeMerkleTree();
  }

  /**
   * @dev Public function to check if the Merkle tree has been initialized.
   * @return bool True if the Merkle tree is initialized, false otherwise.
   */
  function isMerkleTreeInitialized() public view returns (bool) {
    return _isMerkleTreeInitialized();
  }

  /**
   * @notice sets the messenger address.
   * @param messengerAddress The address of the messenger contract that is authorized to append messages.
   */
  function setMessenger(address messengerAddress) external override onlyOwner {
    if (messengerAddress == address(0)) {
      revert ErrorInvalidAddress();
    }
    emit MessengerSet(messenger, messengerAddress);
    messenger = messengerAddress;
  }

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATION FUNCTIONS  
    //////////////////////////////////////////////////////////////////////////*/

  /**
   * @notice Appends a new message hash to the Merkle tree and computes the updated Merkle root.
   * @dev This function can only be called by the authorized messenger contract.
   * @param messageHash The hash of the message to append to the Merkle tree.
   * @return currentNonce The index of the newly appended message in the tree.
   * @return currentRoot The updated Merkle root after appending the new message.
   * @custom:throws "only messenger" if the caller is not the authorized messenger contract.
   */
  function appendMessage(bytes32 messageHash) external override returns (uint256, bytes32) {
    if (_msgSender() != messenger) {
      revert ErrorUnauthorised();
    }
    (uint256 currentNonce, bytes32 currentRoot) = _appendMessageHash(messageHash);
    return (currentNonce, currentRoot);
  }
}
