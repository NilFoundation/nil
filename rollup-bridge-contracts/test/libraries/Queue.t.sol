pragma solidity ^0.8.21;

import "forge-std/Test.sol";
import "../../contracts/bridge/libraries/Queue.sol";

contract QueueTest is Test {
    using Queue for Queue.QueueData;

    Queue.QueueData private queue;

    function setUp() public {
        // Initialize the queue with some messages
        queue.pushBack(keccak256("message1"));
        queue.pushBack(keccak256("message2"));
    }

    function testPushBack() public {
        queue.pushBack(keccak256("message3"));
        assertEq(queue.getSize(), 3);
    }

    function testFront() public {
        bytes32 messageHash = queue.front();
        assertEq(messageHash, keccak256("message1"));
    }

    function testBack() public {
        bytes32 messageHash = queue.back();
        assertEq(messageHash, keccak256("message2"));
    }

    function testPopFront() public {
        bytes32 messageHash = queue.popFront();
        assertEq(messageHash, keccak256("message1"));
        assertEq(queue.getSize(), 1);
    }

    function testPopFrontBatch() public {
        queue.pushBack(keccak256("message3"));
        bytes32[] memory messageHashes = queue.popFrontBatch(2);
        assertEq(messageHashes.length, 2);
        assertEq(messageHashes[0], keccak256("message1"));
        assertEq(messageHashes[1], keccak256("message2"));
        assertEq(queue.getSize(), 1);
    }

    function testGetSize() public {
        assertEq(queue.getSize(), 2);
    }

    function testIsEmpty() public {
        assertFalse(queue.isEmpty());
        queue.popFront();
        queue.popFront();
        assertTrue(queue.isEmpty());
    }

    function testErrorSelector() public {
        bytes4 selector = bytes4(keccak256("NotEnoughMessagesInQueue()"));
        console.logBytes4(selector);
    }

    function testQueueIsEmptyError() public {
        // Empty the queue
        queue.popFront();
        queue.popFront();

        // Expect the QueueIsEmpty error when calling front on an empty queue
        vm.expectRevert(QueueIsEmpty.selector);
        queue.front();

        // Expect the QueueIsEmpty error when calling back on an empty queue
        vm.expectRevert(QueueIsEmpty.selector);
        queue.back();

        // Expect the QueueIsEmpty error when calling popFront on an empty queue
        vm.expectRevert(QueueIsEmpty.selector);
        queue.popFront();
    }

    function testNotEnoughMessagesInQueueError() public {
        // Expect the NotEnoughMessagesInQueue error when calling popFrontBatch with more messages than available
        vm.expectRevert(NotEnoughMessagesInQueue.selector);
        queue.popFrontBatch(3);
    }
}
