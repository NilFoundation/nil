// SPDX-License-Identifier: MIT
pragma solidity 0.8.27;

// 0x63c36549
error QueueIsEmpty();
// 0x3ef0d521
error NotEnoughMessagesInQueue();

/// @dev The library provides the API to interact with the queue container
/// @dev Order of processing operations from queue - FIFO (First in - first out)
library Queue {
    using Queue for QueueData;

    /// @notice Container that stores messageHashes
    /// @param data The inner mapping that saves messageHash by its index
    /// @param head The pointer to the first unprocessed messageHash, equal to the tail if the queue is empty
    /// @param tail The pointer to the free slot
    struct QueueData {
        mapping(uint256 => bytes32) data;
        uint256 tail;
        uint256 head;
    }

    /// @notice Returns zero if and only if no operations were processed from the queue
    /// @return Index of the oldest messageHash that wasn't processed yet
    function getFirstUnprocessedMessageHash(QueueData storage _queue) internal view returns (uint256) {
        return _queue.head;
    }

    /// @return The total number of messages that were added to the queue, including all processed ones
    function getTotalMessageHashes(QueueData storage _queue) internal view returns (uint256) {
        return _queue.tail;
    }

    /// @return The total number of unprocessed messages in the queue
    function getSize(QueueData storage _queue) internal view returns (uint256) {
        return uint256(_queue.tail - _queue.head);
    }

    /// @return Whether the queue contains no messages
    function isEmpty(QueueData storage _queue) internal view returns (bool) {
        return _queue.tail == _queue.head;
    }

    /// @notice Add the messageHash to the end of the queue
    function pushBack(QueueData storage _queue, bytes32 _messageHash) internal {
        uint256 tail = _queue.tail;
        _queue.data[tail] = _messageHash;
        _queue.tail = tail + 1;
    }

    /// @return The first unprocessed messageHash from the queue
    function front(QueueData storage _queue) internal view returns (bytes32) {
        if (_queue.isEmpty()) {
            revert QueueIsEmpty();
        }
        return _queue.data[_queue.head];
    }

    /// @return The last messageHash from the queue
    function back(QueueData storage _queue) internal view returns (bytes32) {
        if (_queue.isEmpty()) {
            revert QueueIsEmpty();
        }
        return _queue.data[_queue.tail - 1];
    }

    /// @notice Remove the first unprocessed messageHash from the queue
    /// @return messageHash that was popped from the queue
    function popFront(QueueData storage _queue) internal returns (bytes32 messageHash) {
        if (_queue.isEmpty()) {
            revert QueueIsEmpty();
        }
        uint256 head = _queue.head;
        messageHash = _queue.data[head];
        delete _queue.data[head];
        _queue.head = head + 1;
    }

    /// @notice Remove the first `n` unprocessed messageHashes from the queue
    /// @param n The number of messageHashes to remove
    /// @return messageHashes The array of messageHashes that were popped from the queue
    function popFrontBatch(QueueData storage _queue, uint256 n) internal returns (bytes32[] memory messageHashes) {
        if(_queue.getSize() < n) {
            revert NotEnoughMessagesInQueue();
        }
        messageHashes = new bytes32[](n);
        for (uint256 i = 0; i < n; i++) {
            messageHashes[i] = _queue.popFront();
        }
    }
}
