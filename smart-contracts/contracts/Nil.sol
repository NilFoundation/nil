// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// TokenId is a type that represents a unique token identifier.
type TokenId is address;

using {
    tokenIdEqual as ==,
    tokenIdNotEqual as !=,
    tokenIdLess as <,
    tokenIdGreater as >,
    tokenIdLessOrEqual as <=,
    tokenIdGreaterOrEqual as >=
} for TokenId global;

library Nil {
    address public constant ASYNC_CALL = address(0xfd);
    address public constant VERIFY_SIGNATURE = address(0xfe);
    address public constant IS_INTERNAL_TRANSACTION = address(0xff);
    address public constant MANAGE_TOKEN = address(0xd0);
    address private constant GET_TOKEN_BALANCE = address(0xd1);
    address private constant SEND_TOKEN_SYNC = address(0xd2);
    address private constant GET_TRANSACTION_TOKENS = address(0xd3);
    address private constant GET_GAS_PRICE = address(0xd4);
    address private constant CONFIG_PARAM = address(0xd7);
    address public constant IS_RESPONSE_TRANSACTION = address(0xd9);
    address public constant LOG = address(0xda);
    address public constant GOVERNANCE = address(0xdb);

    // The following constants specify from where and how the gas should be taken during async call.
    // Forwarding values are calculated in the following order: FORWARD_VALUE, FORWARD_PERCENTAGE, FORWARD_REMAINING.
    //
    // Take whole remaining gas from inbound transaction feeCredit. If there are more than one transactions with such forward
    // kind, the gas will be divided and forwarded in equal parts.
    uint8 public constant FORWARD_REMAINING = 0;
    // Get a percentage of the available feeCredit.
    uint8 public constant FORWARD_PERCENTAGE = 1;
    // Get exact value from the available feeCredit.
    uint8 public constant FORWARD_VALUE = 2;
    // Do not forward gas from inbound transaction, take gas from the account instead.
    uint8 public constant FORWARD_NONE = 3;
    // Minimal amount of gas reserved by asyncCall with response processing.
    uint public constant ASYNC_REQUEST_MIN_GAS = 50_000;

    // Token is a struct that represents a token with an id and amount.
    struct Token {
        TokenId id;
        uint256 amount;
    }

    /**
     * @dev Deploys a contract asynchronously. It implicitly uses FORWARD_REMAINING kind and sets refundTo to
     *  inbound transaction's refundTo.
     * @param shardId The shard ID where to deploy contract.
     * @param bounceTo Address to bounce to if the deployment fails.
     * @param value Value to be sent with the deployment.
     * @param code Bytecode of the contract to be deployed.
     * @param salt Salt for the contract address creation.
     * @return Address of the deployed contract.
     */
    function asyncDeploy(
        uint shardId,
        address bounceTo,
        uint value,
        bytes memory code,
        uint256 salt
    ) internal returns (address) {
        return Nil.asyncDeploy(shardId, address(0), bounceTo, 0, FORWARD_REMAINING, value, code, salt);
    }

    /**
     * @dev Deploys a contract asynchronously.
     * @param shardId The shard ID where to deploy contract.
     * @param refundTo Address to refund if the deployment fails.
     * @param bounceTo Address to bounce to if the deployment fails.
     * @param feeCredit Fee credit for the deployment.
     * @param forwardKind Kind of forwarding for the gas.
     * @param value Value to be sent with the deployment.
     * @param code Bytecode of the contract to be deployed.
     * @param salt Salt for the contract address creation.
     * @return Address of the deployed contract.
     */
    function asyncDeploy(
        uint shardId,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        uint8 forwardKind,
        uint value,
        bytes memory code,
        uint256 salt
    ) internal returns (address) {
        Token[] memory tokens;
        address contractAddress = Nil.createAddress(shardId, code, salt);
        __Precompile__(ASYNC_CALL).precompileAsyncCall{value: value}(true, forwardKind, contractAddress, refundTo,
            bounceTo, feeCredit, tokens, bytes.concat(code, bytes32(salt)), 0, 0);
        return contractAddress;
    }

    /**
     * @dev Makes an asynchronous call to a contract. It implicitly uses FORWARD_REMAINING kind and sets refundTo to
     *  inbound transaction's refundTo.
     * @param dst Destination address of the call.
     * @param bounceTo Address to bounce to if the call fails.
     * @param value Value to be sent with the call.
     * @param callData Calldata for the call.
     */
    function asyncCall(
        address dst,
        address bounceTo,
        uint value,
        bytes memory callData
    ) internal {
        asyncCall(dst, address(0), bounceTo, 0, FORWARD_REMAINING, value, callData);
    }

    /**
     * @dev Makes an asynchronous call to a contract.
     * @param dst Destination address of the call.
     * @param refundTo Address to refund if the call fails.
     * @param bounceTo Address to bounce to if the call fails.
     * @param feeCredit Fee credit for the call.
     * @param forwardKind Kind of forwarding for the gas.
     * @param value Value to be sent with the call.
     * @param callData Calldata for the call.
     */
    function asyncCall(
        address dst,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        uint8 forwardKind,
        uint value,
        bytes memory callData
    ) internal {
        Token[] memory tokens;
        asyncCallWithTokens(dst, refundTo, bounceTo, feeCredit, forwardKind, value, tokens, callData);
    }

    /**
     * @dev Makes an asynchronous call to a contract with tokens.
     * @param dst Destination address of the call.
     * @param refundTo Address to refund if the call fails.
     * @param bounceTo Address to bounce to if the call fails.
     * @param feeCredit Fee credit for the call.
     * @param forwardKind Kind of forwarding for the gas.
     * @param value Value to be sent with the call.
     * @param tokens Array of tokens to be sent with the call.
     * @param callData Calldata for the call.
     */
    function asyncCallWithTokens(
        address dst,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        uint8 forwardKind,
        uint value,
        Token[] memory tokens,
        bytes memory callData
    ) internal {
        __Precompile__(ASYNC_CALL).precompileAsyncCall{value: value}(false, forwardKind, dst, refundTo,
            bounceTo, feeCredit, tokens, callData, 0, 0);
    }

    /**
     * @dev Makes a synchronous call to a contract.
     * @param dst Destination address of the call.
     * @param gas Gas limit for the call.
     * @param value Value to be sent with the call.
     * @param tokens Array of tokens to be sent with the call.
     * @param callData Calldata for the call.
     * @return success Boolean indicating if the call was successful.
     * @return returnData Data returned from the call.
     */
    function syncCall(
        address dst,
        uint gas,
        uint value,
        Token[] memory tokens,
        bytes memory callData
    ) internal returns(bool, bytes memory) {
        if (tokens.length > 0) {
            __Precompile__(SEND_TOKEN_SYNC).precompileSendTokens(dst, tokens);
        }
        (bool success, bytes memory returnData) = dst.call{gas: gas, value: value}(callData);
        return (success, returnData);
    }

    /**
     * @dev Validates a signature using a precompiled contract.
     * @param pubkey Public key used for validation.
     * @param hash Hash of the transaction.
     * @param signature Signature to be validated.
     * @return Boolean indicating if the signature is valid.
     */
    function validateSignature(
        bytes memory pubkey,
        uint256 hash,
        bytes memory signature
    ) internal view returns (bool) {
        // ABI encode the input parameters
        bytes memory encodedInput = abi.encode(pubkey, hash, signature);
        bool success;
        bool result;

        // Perform the static call to the precompiled contract at address `VerifyExternalTransaction`
        bytes memory returnData;
        (success, returnData) = VERIFY_SIGNATURE.staticcall(encodedInput);

        require(success, "Precompiled contract call failed");

        // Extract the boolean result from the returned data
        if (returnData.length > 0) {
            result = abi.decode(returnData, (bool));
        }

        return result;
    }

    /**
     * @dev Returns the balance of a token with a given id for a given address.
     * @param addr Address to check the balance for.
     * @param id TokenId of the token.
     * @return Balance of the token.
     */
    function tokenBalance(address addr, TokenId id) internal view returns(uint256) {
        return __Precompile__(GET_TOKEN_BALANCE).precompileGetTokenBalance(id, addr);
    }

    /**
     * @dev Returns tokens from the current transaction.
     * @return Array of tokens from the current transaction.
     */
    function txnTokens() internal returns(Token[] memory) {
        return __Precompile__(GET_TRANSACTION_TOKENS).precompileGetTransactionTokens();
    }

    /**
     * @dev Returns the shard id for a given address.
     * @param addr Address to get the shard id for.
     * @return Shard id of the address.
     */
    function getShardId(address addr) internal pure returns(uint256) {
        return uint256(uint160(addr)) >> (18 * 8);
    }

    /**
     * @notice Returns the gas price for the shard in which the given address resides.
     * @dev It may return the price with some delay, i.e it can be not equal to the actual price. So, one should
     * calculate real gas price pessimistically, i.e. `gas_price = getGasPrice() + blocks_delay * price_growth_factor`.
     * Where, `blocks_delay` is the blocks number between the block for which gas price is actual and the block in which
     * the transaction will be processed; and `price_growth_factor` is the maximum value by which gas can grow per block.
     * @param addr Address to get the gas price for.
     * @return Gas price for the shard.
     */
    function getGasPrice(address addr) internal returns(uint256) {
        return __Precompile__(GET_GAS_PRICE).precompileGetGasPrice(getShardId(addr));
    }

    /**
     * @dev Creates a contract address.
     * @param shardId Shard id for the address.
     * @param code Bytecode of the contract.
     * @param salt Salt for the address creation.
     * @return Address of the created contract.
     */
    function createAddress(uint shardId, bytes memory code, uint256 salt) internal pure returns(address) {
        require(shardId < 0xffff, "Shard id is too big");
        uint160 addr = uint160(uint256(keccak256(abi.encodePacked(code, salt))));
        addr &= 0xffffffffffffffffffffffffffffffffffff;
        addr |= uint160(shardId) << (18 * 8);
        return address(addr);
    }

    /**
     * @dev Creates a contract address using create2.
     * @param shardId Shard id for the address.
     * @param sender Address of the sender.
     * @param salt Salt for the address creation.
     * @param codeHash Hash of the contract bytecode.
     * @return Address of the created contract.
     */
    function createAddress2(uint shardId, address sender, uint256 salt, uint256 codeHash) internal pure returns(address) {
        require(shardId < 0xffff, "Shard id is too big");
        uint160 addr = uint160(uint256(keccak256(abi.encodePacked(bytes1(0xff), sender, salt, codeHash))));
        addr &= 0xffffffffffffffffffffffffffffffffffff;
        addr |= uint160(shardId) << (18 * 8);
        return address(addr);
    }

    /**
     * @dev Initiates a rollback
     *
     * Unused parameters are temporarily commented out.
     */
    function rollback(
        uint32 version,
        uint32 counter,
        uint32 patchLevel,
        uint64 mainBlockId /*,
        uint32 replayDepth,
        uint32 searchDepth */
    ) internal {
        __Precompile__(GOVERNANCE).precompileRollback(version, counter, patchLevel, mainBlockId /*, replayDepth, searchDepth */);
    }

    /**
     * @dev Sets a configuration parameter.
     * @param name Name of the parameter.
     * @param data Data of the parameter.
     */
    function setConfigParam(string memory name, bytes memory data) internal {
        __Precompile__(CONFIG_PARAM).precompileConfigParam(true, name, data);
    }

    /**
     * @dev Returns a configuration parameter.
     * @param name Name of the parameter.
     * @return Data of the parameter.
     */
    function getConfigParam(string memory name) internal returns(bytes memory) {
        return __Precompile__(CONFIG_PARAM).precompileConfigParam(false, name, bytes(""));
    }

    struct ValidatorInfo {
        uint8[33] PublicKey;
        address WithdrawalAddress;
    }

    struct ListValidators{
        ValidatorInfo[] list;
    }

    struct ParamValidators {
        ListValidators[] validators;
    }

    struct ParamGasPrice {
        uint256[] shards;
    }

    struct ParamL1BlockInfo {
        uint64 number;
        uint64 timestamp;
        uint256 baseFee;
        uint256 blobBaseFee;
        bytes32 hash;
    }

    /**
     * @dev Returns the current validators.
     * @return Struct containing the list of validators.
     */
    function getValidators() internal returns(ParamValidators memory) {
        bytes memory data = getConfigParam("curr_validators");
        return abi.decode(data, (ParamValidators));
    }

    /**
     * @dev Returns the gas price parameter.
     * @return Struct containing the gas price scale.
     */
    function getParamGasPrice() internal returns(ParamGasPrice memory) {
        bytes memory data = getConfigParam("gas_price");
        return abi.decode(data, (ParamGasPrice));
    }

    /**
     * @dev Logs a transaction with data.
     * @param transaction Transaction to log.
     * @param data Data to log.
     */
    function log(string memory transaction, int[] memory data) internal {
        __Precompile__(LOG).precompileLog(transaction, data);
    }

    /**
     * @dev Logs a transaction with empty data.
     * @param transaction Transaction to log.
     */
    function log(string memory transaction) internal {
        int[] memory data;
        __Precompile__(LOG).precompileLog(transaction, data);
    }
}

// NilBase is a base contract that provides modifiers for checking the type of transaction (internal or external).
contract NilBase {
    /**
     * @dev Modifier to check that the method was invoked from a response transaction.
     */
    modifier onlyResponse() {
        (bool success,/* bytes memory returnData*/) = Nil.IS_RESPONSE_TRANSACTION.staticcall(bytes(""));
        require(success, "IS_RESPONSE_TRANSACTION call failed");
        _;
    }

    /**
     * @dev Modifier to check that the method was invoked from an internal transaction.
     */
    modifier onlyInternal() {
        require(isInternalTransaction(), "Trying to call internal function with external transaction");
        _;
    }

    /**
     * @dev Modifier to check that the method was invoked from an external transaction.
     */
    modifier onlyExternal() {
        require(!isInternalTransaction(), "Trying to call external function with internal transaction");
        _;
    }

    // isInternalTransaction returns true if the current transaction is internal.
    function isInternalTransaction() internal view returns (bool) {
        bytes memory data;
        (bool success, bytes memory returnData) = Nil.IS_INTERNAL_TRANSACTION.staticcall(data);
        require(success, "Precompiled contract call failed");
        require(returnData.length > 0, "'IS_INTERNAL_TRANSACTION' returns invalid data");
        return abi.decode(returnData, (bool));
    }
}

abstract contract NilBounceable is NilBase {
    function bounce(string calldata err) virtual payable external;
}

// WARNING: User should never use this contract directly.
contract __Precompile__ {
    // if mint flag is set to false, token will be burned instead
    function precompileManageToken(uint256 amount, bool mint) public returns(bool) {}
    function precompileGetTokenBalance(TokenId id, address addr) public view returns(uint256) {}
    function precompileAsyncCall(bool, uint8, address, address, address, uint, Nil.Token[] memory, bytes memory, uint256, uint) public payable returns(bool) {}
    function precompileSendTokens(address, Nil.Token[] memory) public returns(bool) {}
    function precompileGetTransactionTokens() public returns(Nil.Token[] memory) {}
    function precompileGetGasPrice(uint id) public returns(uint256) {}
    function precompileConfigParam(bool isSet, string calldata name, bytes calldata data) public returns(bytes memory) {}
    function precompileLog(string memory transaction, int[] memory data) public returns(bool) {}
    function precompileRollback(uint32, uint32, uint32, uint64 /*, uint32, uint32*/) public returns(bool) {}
}

contract NilConfigAbi {
    function curr_validators(Nil.ParamValidators memory) public {}
    function gas_price(Nil.ParamGasPrice memory) public {}
    function l1block(Nil.ParamL1BlockInfo memory) public {}
}

function tokenIdEqual(TokenId a, TokenId b) pure returns (bool) {
    return TokenId.unwrap(a) == TokenId.unwrap(b);
}

function tokenIdNotEqual(TokenId a, TokenId b) pure returns (bool) {
    return TokenId.unwrap(a) != TokenId.unwrap(b);
}

function tokenIdLess(TokenId a, TokenId b) pure returns (bool) {
    return TokenId.unwrap(a) < TokenId.unwrap(b);
}

function tokenIdGreater(TokenId a, TokenId b) pure returns (bool) {
    return TokenId.unwrap(a) > TokenId.unwrap(b);
}

function tokenIdLessOrEqual(TokenId a, TokenId b) pure returns (bool) {
    return TokenId.unwrap(a) <= TokenId.unwrap(b);
}

function tokenIdGreaterOrEqual(TokenId a, TokenId b) pure returns (bool) {
    return TokenId.unwrap(a) >= TokenId.unwrap(b);
}
