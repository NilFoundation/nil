pragma solidity ^0.8.28;

interface INilMessageTree {
  error ErrorInvalidAddress();

  event MessengerSet(address indexed oldMessenger, address indexed newMessenger);

  function setMessenger(address messengerAddress) external;

  function appendMessage(bytes32 _messageHash) external returns (uint256, bytes32);

  function isMerkleTreeInitialized() external view returns (bool);
}
