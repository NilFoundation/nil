pragma solidity ^0.8.28;

import "forge-std/Test.sol";
import "../../contracts/bridge/libraries/Queue.sol";
import "./QueueTest.sol";

contract MessageQueueTest is Test {
    QueueTest private queueTest;

    function setUp() public {
        queueTest = new QueueTest();
    }

    function testPushBack() public {
        queueTest.pushBack(keccak256("message1"));
        queueTest.pushBack(keccak256("message2"));
        queueTest.pushBack(keccak256("message3"));
        assertEq(queueTest.getSize(), 3);
    }

    function testFront() public {
        queueTest.pushBack(keccak256("message1"));
        queueTest.pushBack(keccak256("message2"));
        bytes32 messageHash = queueTest.front();
        assertEq(messageHash, keccak256("message1"));
    }

    function testPopFront() public {
        queueTest.pushBack(keccak256("message1"));
        queueTest.pushBack(keccak256("message2"));
        bytes32 messageHash = queueTest.popFront();
        assertEq(messageHash, keccak256("message1"));
        assertEq(queueTest.getSize(), 1);
    }

    function testPopFrontBatch() public {
        queueTest.pushBack(keccak256("message1"));
        queueTest.pushBack(keccak256("message2"));
        queueTest.pushBack(keccak256("message3"));
        bytes32[] memory messageHashes = queueTest.popFrontBatch(2);
        assertEq(messageHashes.length, 2);
        assertEq(messageHashes[0], keccak256("message1"));
        assertEq(messageHashes[1], keccak256("message2"));
        assertEq(queueTest.getSize(), 1);
    }

    function testGetSize() public {
        queueTest.pushBack(keccak256("message1"));
        queueTest.pushBack(keccak256("message2"));
        assertEq(queueTest.getSize(), 2);
    }

    function testIsEmpty() public {
        queueTest.pushBack(keccak256("message1"));
        queueTest.pushBack(keccak256("message2"));
        assertFalse(queueTest.isEmpty());
        queueTest.popFront();
        queueTest.popFront();
        assertTrue(queueTest.isEmpty());
    }

    function testQueueIsEmptyError() public {
        queueTest.pushBack(keccak256("message1"));
        queueTest.pushBack(keccak256("message2"));
        queueTest.popFront();
        queueTest.popFront();

        vm.expectRevert(QueueIsEmpty.selector);
        queueTest.front();

        vm.expectRevert(QueueIsEmpty.selector);
        queueTest.popFront();
    }

    function testNotEnoughMessagesInQueueError() public {
        queueTest.pushBack(keccak256("message1"));
        vm.expectRevert(NotEnoughMessagesInQueue.selector);
        queueTest.popFrontBatch(2);
    }
}
