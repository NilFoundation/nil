pragma solidity 0.8.28;

interface IAppendOnlyMerkleTree {
  function messageRoot() external view returns (bytes32);

  function nextMessageIndex() external view returns (uint256);
}
