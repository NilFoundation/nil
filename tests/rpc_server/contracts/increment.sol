pragma solidity ^0.8.0;

contract Incrementer {
    uint256 private value;

    function increment_and_send_msg() public {
        // types.Message{
        //     Seqno: 123,
        //     From:  common.BytesToAddress([]byte("from-address")),
        //     To:    common.BytesToAddress([]byte("to-address")),
        // }).MarshalSSZ()
        bytes32[5] memory input = [
            bytes32(hex'7b00000000000000000000000000000000000000000000000000000000000000'),
            bytes32(hex'0000000000000000000000000000000000000000000000000000000000000000'),
            bytes32(hex'0000000000000000000000000000000066726f6d2d6164647265737300000000'),
            bytes32(hex'000000000000746f2d6164647265737300000000000000000000000000000000'),
            bytes32(hex'00000000000000000000000000000000d5000000000000000000000000000000')
        ];

        uint[1] memory output;

        value += 1;

        assembly {
            if iszero(call(
                /* gas = */              not(0),
                /* contract address = */ 0x06,
                /* value = */            0,
                /* input mem start = */  input,
                /* input mem size = */   213,
                /* output mem start */   output,
                /* output mem size */    0)) {
                revert(0, 0)
            }
        }
    }
}
