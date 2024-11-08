// SPDX-License-Identifier: MIT

pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Retailer {
    using Nil for address;

    receive() external payable {}

    function orderProduct(address dst, string calldata name) public {
        dst.asyncCall(
            msg.sender,
            0,
            abi.encodeWithSignature("createProduct(string)", name)
        );
    }

    function verifyExternal(
        uint256 hash,
        bytes memory _authData
    ) external view returns (bool) {
        return true;
    }
}

contract Manufacturer is NilBase {
    using Nil for address;

    bytes pubkey;
    address retailerContractAddress;

    receive() external payable {}

    constructor(
        bytes memory pubkeyOne,
        address _retailerContractAddress
    ) payable {
        pubkey = pubkeyOne;
        retailerContractAddress = _retailerContractAddress;
    }

    struct Product {
        uint id;
        string name;
    }

    mapping(uint => Product) public products;
    uint public nextProductId;

    function createProduct(
        string calldata productName
    ) public onlyInternal returns (bool) {
        if (msg.sender == retailerContractAddress) {
            products[nextProductId] = Product(nextProductId, productName);
            nextProductId++;
            return true;
        }
        return false;
    }

    function verifyExternal(
        uint256 hash,
        bytes calldata signature
    ) external view returns (bool) {
        return Nil.validateSignature(pubkey, hash, signature);
    }

    function getProducts()
        public
        view
        returns (uint[] memory, string[] memory)
    {
        uint[] memory ids = new uint[](nextProductId);
        string[] memory names = new string[](nextProductId);

        for (uint i = 0; i < nextProductId; i++) {
            Product storage product = products[i];
            ids[i] = product.id;
            names[i] = product.name;
        }

        return (ids, names);
    }
}
