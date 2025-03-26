// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "./VoteShard.sol";

/**
 * @title VoteManager
 * @dev Manages deployment of voting shards and coordination of vote casting and tallying across shards.
 */
contract VoteManager is NilBase, Ownable {
    uint256 public numShards;
    uint16 public numChoices;
    uint256 public startTime;
    uint256 public endTime;

    mapping(uint256 => address) public voteShards;
    mapping(uint16 => uint256) public results;

    event DeploymentComplete(uint256 _numShards, uint256 _numChoices);
    event VotesTallied(uint256 _shardId, address _voteShard);

    /**
     * @dev Initializes the voting manager with shard and voting configuration.
     * @param _numShards Number of vote shards to deploy.
     * @param _numChoices Number of voting options.
     * @param _startTime Timestamp when voting starts.
     * @param _endTime Timestamp when voting ends.
     */
    constructor(uint256 _numShards, uint16 _numChoices, uint256 _startTime, uint256 _endTime)
        payable
        Ownable(msg.sender)
    {
        require(_numShards > 0, "Shards start from 1");
        require(_startTime >= block.timestamp, "Start time must be in the future");
        require(_endTime > _startTime, "End time must be after start time");
        require(_numChoices > 0, "Choices start from 1");

        numShards = _numShards;
        numChoices = _numChoices;
        startTime = _startTime;
        endTime = _endTime;
    }

    /**
     * @notice Deploys all vote shard contracts deterministically using CREATE2.
     */
    function deployVotingShards() public payable {
        for (uint256 i = 1; i <= numShards; i++) {
            uint256 salt = generateSalt(msg.sender, i);
            deploy(i, salt);
        }

        emit DeploymentComplete(numShards, numChoices);
    }

    /**
     * @dev Deploys a single VoteShard contract using Nil.asyncDeploy.
     * @param _shardId ID of the shard.
     * @param _salt Deterministic salt for CREATE2 deployment.
     */
    function deploy(uint256 _shardId, uint256 _salt) private payable {
        bytes memory data =
            bytes.concat(type(VoteShard).creationCode, abi.encode(address(this), numChoices, startTime, endTime));
        address voteShard = Nil.asyncDeploy(_shardId, address(this), 0, data, _salt);
        voteShards[_shardId] = voteShard;
    }

    /**
     * @notice Allows users to vote through the VoteManager contract.
     * @param _shardId ID of the VoteShard to cast the vote in.
     * @param _choice Voter's selected option.
     */
    function voteFromManager(uint256 _shardId, uint16 _choice) public payable {
        require(_choice > 0 && _choice <= numChoices, "Invalid choice");
        require(_shardId > 0 && _shardId <= numShards, "Invalid shard ID");

        address voteShard = voteShards[_shardId];
        bytes memory callData = abi.encodeWithSignature("vote(uint16,address)", _choice, msg.sender);

        Nil.asyncCall(voteShard, msg.sender, 0, callData);
    }

    /**
     * @notice Initiates asynchronous tallying across all voting shards.
     */
    function tallyTotalVotes() public payable {
        for (uint256 i = 1; i <= numShards; i++) {
            address voteShard = voteShards[i];

            bytes memory callData = abi.encodeWithSignature("tallyVotes()");
            bytes memory context = abi.encodeWithSelector(this.processVotesTally.selector, i, voteShard);

            Nil.sendRequest(voteShard, 0, 9_000_000, context, callData);
        }
    }

    /**
     * @notice Processes tally results returned from a VoteShard.
     * @param _success Whether the request succeeded.
     * @param _returnData Encoded results of tallyVotes().
     * @param _context Encoded shardId and voteShard address.
     */
    function processVotesTally(bool _success, bytes memory _returnData, bytes memory _context) public {
        require(_success, "Vote tally call failed");

        (uint256 shardId, address voteShard) = abi.decode(_context, (uint256, address));
        uint256[] memory votes = abi.decode(_returnData, (uint256[]));

        for (uint16 i = 1; i <= numChoices; i++) {
            results[i] += votes[i];
        }

        emit VotesTallied(shardId, voteShard);
    }

    /**
     * @notice Verification hook required by Nil runtime.
     */
    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }

    /**
     * @dev Generates a deterministic salt using address and nonce.
     * @param _user Address to base the salt on.
     * @param _nonce A nonce value to ensure uniqueness.
     */
    function generateSalt(address _user, uint256 _nonce) private pure returns (uint256) {
        return uint256(keccak256(abi.encodePacked(_user, _nonce)));
    }
}
