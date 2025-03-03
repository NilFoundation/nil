pragma solidity ^0.8.0;

contract GasBurner {
    uint256[] public data;

    function burnGas() public payable {
        data = new uint256[](2**24);
        require(false, "Intentional failure");
    }
}
