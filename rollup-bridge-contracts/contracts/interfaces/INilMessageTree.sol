pragma solidity 0.8.28;

interface INilMessageTree {
  function appendMessage(bytes32 _messageHash) external returns (uint256, bytes32);
}
