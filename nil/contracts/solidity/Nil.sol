// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

library Nil {
    uint private constant SEND_MESSAGE = 0xfc;
    address private constant ASYNC_CALL = address(0xfd);
    address public constant VERIFY_SIGNATURE = address(0xfe);
    address public constant IS_INTERNAL_MESSAGE = address(0xff);
    address public constant MINT_CURRENCY = address(0xd0);
    address private constant GET_CURRENCY_BALANCE = address(0xd1);
    address private constant SEND_CURRENCY_SYNC = address(0xd2);
    address private constant GET_MESSAGE_TOKENS = address(0xd3);
    address private constant GET_GAS_PRICE = address(0xd4);
    address private constant GET_POSEIDON_HASH = address(0xd5);
    address private constant AWAIT_CALL = address(0xd6);
    address private constant CONFIG_PARAM = address(0xd7);

    // The following constants specify from where and how the gas should be taken during async call.
    // Forwarding values are calculated in the following order: FORWARD_VALUE, FORWARD_PERCENTAGE, FORWARD_REMAINING.
    //
    // Take whole remaining gas from inbound message feeCredit. If there are more than one messages with such forward
    // kind, the gas will be divided and forwarded in equal parts.
    uint8 public constant FORWARD_REMAINING = 0;
    // Get a percentage of the available feeCredit.
    uint8 public constant FORWARD_PERCENTAGE = 1;
    // Get exact value from the available feeCredit.
    uint8 public constant FORWARD_VALUE = 2;
    // Do not forward gas from inbound message, take gas from the account instead.
    uint8 public constant FORWARD_NONE = 3;

    // Token is a struct that represents a token with an id and amount.
    struct Token {
        uint256 id;
        uint256 amount;
    }

    // Concise version of asyncCall. It implicitly uses FORWARD_REMAINING kind and sets refundTo to inbound message's
    // refundTo.
    function asyncCall(
        address dst,
        address bounceTo,
        uint value,
        bytes memory callData
    ) internal {
        Token[] memory tokens;
        asyncCall(dst, address(0), bounceTo, 0, FORWARD_REMAINING, false, value, tokens, callData);
    }

    // awaitCall makes an asynchronous call to `dst` contract and waits for the result.
    function awaitCall(
        address dst,
        bytes memory callData
    ) internal returns(bytes memory, bool) {
        return __Precompile__(AWAIT_CALL).precompileAwaitCall(dst, callData);
    }

    // asyncCall makes an asynchronous call to `dst` contract.
    function asyncCall(
        address dst,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        uint8 forwardKind,
        bool deploy,
        uint value,
        bytes memory callData
    ) internal {
        Token[] memory tokens;
        asyncCall(dst, refundTo, bounceTo, feeCredit, forwardKind, deploy, value, tokens, callData);
    }

    // asyncCall makes an asynchronous call to `dst` contract.
    function asyncCall(
        address dst,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        uint8 forwardKind,
        bool deploy,
        uint value,
        Token[] memory tokens,
        bytes memory callData
    ) internal {
        __Precompile__(ASYNC_CALL).precompileAsyncCall{value: value}(deploy, forwardKind, dst, refundTo,
            bounceTo, feeCredit, tokens, callData);
    }

    // asyncCall makes an asynchronous call to `dst` contract.
    function asyncCall(
        address dst,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        bool deploy,
        uint value,
        Token[] memory tokens,
        bytes memory callData
    ) internal {
        __Precompile__(ASYNC_CALL).precompileAsyncCall{value: value}(deploy, FORWARD_NONE, dst, refundTo,
            bounceTo, feeCredit, tokens, callData);
    }

    function syncCall(
        address dst,
        uint gas,
        uint value,
        Token[] memory tokens,
        bytes memory callData
    ) internal returns(bool, bytes memory) {
        if (tokens.length > 0) {
            __Precompile__(SEND_CURRENCY_SYNC).precompileSendTokens(dst, tokens);
        }
        (bool success, bytes memory returnData) = dst.call{gas: gas, value: value}(callData);
        return (success, returnData);
    }

    // Send raw internal message using a special precompiled contract
    function sendMessage(uint g, bytes memory message) internal {
        uint message_size = message.length;
        assembly {
            // Call precompiled contract.
            // Arguments: gas, precompiled address, value, input, input size, output, output size
            if iszero(call(g, SEND_MESSAGE, 0, add(message, 32), message_size, 0, 0)) {
                revert(0, 0)
            }
        }
    }

    // Function to call the validateSignature precompiled contract
    function validateSignature(
        bytes memory pubkey,
        uint256 hash,
        bytes memory signature
    ) internal view returns (bool) {
        // ABI encode the input parameters
        bytes memory encodedInput = abi.encode(pubkey, hash, signature);
        bool success;
        bool result;

        // Perform the static call to the precompiled contract at address `VerifyExternalMessage`
        bytes memory returnData;
        (success, returnData) = VERIFY_SIGNATURE.staticcall(encodedInput);

        require(success, "Precompiled contract call failed");

        // Extract the boolean result from the returned data
        if (returnData.length > 0) {
            result = abi.decode(returnData, (bool));
        }

        return result;
    }

    // getCurrencyBalance returns the balance of a token with a given id for a given address.
    function currencyBalance(address addr, uint256 id) internal view returns(uint256) {
        return __Precompile__(GET_CURRENCY_BALANCE).precompileGetCurrencyBalance(id, addr);
    }

    // msgTokens returns tokens from the current message.
    function msgTokens() internal returns(Token[] memory) {
        return __Precompile__(GET_MESSAGE_TOKENS).precompileGetMessageTokens();
    }

    // getShardId returns shard id for a given address.
    function getShardId(address addr) internal pure returns(uint256) {
        return uint256(uint160(addr)) >> (18 * 8);
    }

    // getGasPrice returns gas price for the shard, in which the given address is resided.
    // It may return the price with some delay, i.e it can be not equal to the actual price. So, one should calculate
    // real gas price pessimistically, i.e. `gas_price = getGasPrice() + blocks_delay * price_growth_factor`.
    // Where, `blocks_delay` is the blocks number between the block for which gas price is actual and the block in which
    // the message will be processed; and `price_growth_factor` is the maximum value by which gas can grow per block.
    // TODO: add `getEstimatedGasPrice` method, which implements the above formula.
    function getGasPrice(address addr) internal returns(uint256) {
        return __Precompile__(GET_GAS_PRICE).precompileGetGasPrice(getShardId(addr));
    }

    function createAddress(uint shardId, bytes memory code, uint256 salt) internal returns(address) {
        require(shardId < 0xffff, "Shard id is too big");
        uint160 addr = uint160(uint256(getPoseidonHash(abi.encodePacked(code, salt))));
        addr &= 0xffffffffffffffffffffffffffffffffffff;
        addr |= uint160(shardId) << (18 * 8);
        return address(addr);
    }

    function createAddress2(uint shardId, address sender, uint256 salt, uint256 codeHash) internal returns(address) {
        require(shardId < 0xffff, "Shard id is too big");
        uint160 addr = uint160(uint256(getPoseidonHash(abi.encodePacked(bytes1(0xff), sender, salt, codeHash))));
        addr &= 0xffffffffffffffffffffffffffffffffffff;
        addr |= uint160(shardId) << (18 * 8);
        return address(addr);
    }

    function getPoseidonHash(bytes memory data) internal returns(uint256) {
        return __Precompile__(GET_POSEIDON_HASH).precompileGetPoseidonHash(data);
    }

    function setConfigParam(string memory name, bytes memory data) internal {
        __Precompile__(CONFIG_PARAM).precompileConfigParam(true, name, data);
    }

    function getConfigParam(string memory name) internal returns(bytes memory) {
        return __Precompile__(CONFIG_PARAM).precompileConfigParam(false, name, bytes(""));
    }

    struct ValidatorInfo {
        uint8[33] PublicKey;
        address WithdrawalAddress;
    }

    struct ParamValidators {
        ValidatorInfo[] list;
    }

    struct ParamGasPrice {
        uint256 gasPriceScale;
    }

    function setValidators(ParamValidators memory validators) internal {
        bytes memory data = abi.encode(validators);
        setConfigParam("curr_validators", data);
    }

    function getValidators() internal returns(ParamValidators memory) {
        bytes memory data = getConfigParam("curr_validators");
        return abi.decode(data, (ParamValidators));
    }

    function setParamGasPrice(ParamGasPrice memory param) internal {
        bytes memory data = abi.encode(param);
        setConfigParam("gas_price", data);
    }

    function getParamGasPrice() internal returns(ParamGasPrice memory) {
        bytes memory data = getConfigParam("gas_price");
        return abi.decode(data, (ParamGasPrice));
    }
}

// NilBase is a base contract that provides modifiers for checking the type of message (internal or external).
contract NilBase {
    // onlyInternal checks that method was invoked from internal message.
    modifier onlyInternal() {
        require(isInternalMessage(), "Trying to call internal function with external message");
        _;
    }

    // onlyExternal checks that method was invoked from external message.
    modifier onlyExternal() {
        require(!isInternalMessage(), "Trying to call external function with internal message");
        _;
    }

    // isInternalMessage returns true if the current message is internal.
    function isInternalMessage() internal view returns (bool) {
        bytes memory data;
        (bool success, bytes memory returnData) = Nil.IS_INTERNAL_MESSAGE.staticcall(data);
        require(success, "Precompiled contract call failed");
        require(returnData.length > 0, "'IS_INTERNAL_MESSAGE' returns invalid data");
        return abi.decode(returnData, (bool));
    }
}

abstract contract NilBounceable is NilBase {
    function bounce(string calldata err) virtual payable external;
}

// WARNING: User should never use this contract directly.
contract __Precompile__ {
    function precompileMintCurrency(uint256 amount) public returns(bool) {}
    function precompileGetCurrencyBalance(uint256 id, address addr) public view returns(uint256) {}
    function precompileAsyncCall(bool, uint8, address, address, address, uint, Nil.Token[] memory, bytes memory) public payable returns(bool) {}
    function precompileAwaitCall(address, bytes memory) public payable returns(bytes memory, bool) {}
    function precompileSendTokens(address, Nil.Token[] memory) public returns(bool) {}
    function precompileGetMessageTokens() public returns(Nil.Token[] memory) {}
    function precompileGetGasPrice(uint id) public returns(uint256) {}
    function precompileGetPoseidonHash(bytes memory data) public returns(uint256) {}
    function precompileConfigParam(bool isSet, string calldata name, bytes calldata data) public returns(bytes memory) {}
}

contract NilConfigAbi {
    function curr_validators(Nil.ParamValidators memory) public {}
    function gas_price(Nil.ParamGasPrice memory) public {}
}
