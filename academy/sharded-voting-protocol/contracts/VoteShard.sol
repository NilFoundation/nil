// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

/**
 * @title VoteShard
 * @notice A shard contract responsible for collecting and tallying votes for a subset of users.
 * @dev Designed to be deployed multiple times via a central VoteManager, each instance handles voting within a specific shard.
 */
contract VoteShard is NilBase {
    /// @notice The total number of voting choices available.
    uint16 public numChoices;

    /// @notice The voting start timestamp.
    uint256 public startTime;

    /// @notice The voting end timestamp.
    uint256 public endTime;

    /// @notice The address of the VoteManager that deployed this shard.
    address public voteManager;

    /// @notice Accepts native NIL tokens.
    receive() external payable {}

    /// @dev Tracks the option chosen by each voter.
    mapping(address => uint16) public options;

    /// @dev Tracks whether a voter has already voted.
    mapping(address => bool) public hasVoted;

    /// @dev Tracks the total number of votes for each option.
    mapping(uint16 => uint256) public totalVotes;

    /// @notice Emitted when a vote is successfully cast.
    event VoteCast(address indexed voter, uint16 indexed choice);

    /// @notice Emitted when voting ends.
    event VotingClosed(uint256 endTime);

    /**
     * @notice Initializes the voting shard with necessary configuration.
     * @param _voteManager The address of the VoteManager contract.
     * @param _numChoices The number of valid choices (must be ≥ 1).
     * @param _startTime Timestamp at which voting opens.
     * @param _endTime Timestamp at which voting closes.
     */
    constructor(address _voteManager, uint16 _numChoices, uint256 _startTime, uint256 _endTime) {
        require(_startTime >= block.timestamp, "Start time must be in the future");
        require(_endTime > _startTime, "End time must be after start time");
        require(_numChoices > 0, "Choices start from 1");

        numChoices = _numChoices;
        startTime = _startTime;
        endTime = _endTime;
        voteManager = _voteManager;
    }

    /**
     * @notice Initiates a vote from a user for a specific choice.
     * @dev Sends an asynchronous request to the VoteManager to check if the voter has voted on other shards.
     * @param _choice The selected voting option (1-based index).
     */
    function vote(uint16 _choice) public {
        require(_choice > 0 && _choice <= numChoices, "Invalid choice");
        address voter = msg.sender;
        require(!hasVoted[voter], "Already voted in this shard");

        // Identify current shard to skip when checking other shards
        uint256 currShardId = Nil.getShardId(address(this));

        // Prepare cross-shard call to check if user has voted elsewhere
        bytes memory callData = abi.encodeWithSignature("checkOtherShards(uint256,address)", currShardId, voter);

        // Context includes voter and selected choice
        bytes memory context = abi.encodeWithSelector(this.processVote.selector, _choice, voter);

        // Send async request to VoteManager
        Nil.sendRequest(voteManager, 0, 9_000_000, context, callData);
    }

    /**
     * @notice Handles the result of the async vote eligibility check.
     * @dev Called by Nil once the cross-shard check completes.
     * @param success Whether the async call was successful.
     * @param returnData The return value from VoteManager (encoded bool).
     * @param context Encoded context passed from the vote() call (choice and voter).
     */
    function processVote(bool success, bytes memory returnData, bytes memory context) public payable {
        require(success, "Cross-shard check failed");

        (uint16 choice, address voter) = abi.decode(context, (uint16, address));
        bool canVote = abi.decode(returnData, (bool));

        require(canVote, "Voter has already voted in another shard");

        options[voter] = choice;
        hasVoted[voter] = true;
        totalVotes[choice] += 1;

        emit VoteCast(voter, choice);
    }

    /**
     * @notice Returns the number of votes for each choice.
     * @dev Index 0 is unused for readability; valid choices start from index 1.
     * @return results An array of total vote counts, where results[i] = votes for choice i.
     */
    function tallyVotes() external view returns (uint256[] memory results) {
        results = new uint256[](numChoices + 1); // index 0 unused

        for (uint16 i = 1; i <= numChoices; i++) {
            results[i] = totalVotes[i];
        }
    }

    /**
     * @notice Checks if a voter has already voted on this shard.
     * @param voter The address of the voter to check.
     * @return True if the voter has already voted in this shard.
     */
    function checkHasVoted(address voter) external view returns (bool) {
        return hasVoted[voter];
    }

    /**
     * @notice Restricts execution to the active voting period.
     * @dev Throws if called before start or after end time.
     */
    modifier duringVote() {
        require(block.timestamp >= startTime && block.timestamp <= endTime, "Voting is not active");
        _;
    }
}
