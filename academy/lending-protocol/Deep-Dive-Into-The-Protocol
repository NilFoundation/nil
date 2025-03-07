### **Sharded Lending Protocol on =nil; Foundation**

**This document is an example and is provided for educational purposes only.** It is not audited, and the implementation described here is not intended for production use. The following guide demonstrates how to implement a decentralized lending and borrowing protocol on **=nil; Foundation** using **sharded smart contract architecture**. The goal is to **illustrate key coding practices**, such as using `sendRequest`, `asyncCall`, `sendRequestWithTokens`, encoding function signatures, and handling `TokenId` as an argument of type `address`. The example should serve as a reference for learning and prototyping.

---

### **Sharded Smart Contract Architecture in DeFi**

**Sharded smart contract design** involves splitting the logic of a decentralized application (dApp) into multiple, independent contracts, each responsible for specific tasks. These contracts communicate asynchronously, enabling the system to process transactions concurrently and increase scalability. In the context of a decentralized lending protocol, this means dividing the protocol’s functions into separate contracts such as **GlobalLedger**, **InterestManager**, **LendingPool**, and **Oracle**. These contracts are **deployed across different shards** to process tasks in parallel, improving system performance and throughput.

This example demonstrates how sharded contracts can enhance scalability, efficiency, and maintainability, but note that the code is meant for educational use only. It has not been audited and should not be used in a production environment without thorough review.

---

### **Key Coding Practices for Implementing the Lending Protocol**

The following section describes best practices used throughout this educational example. We will walk through how to handle key aspects like `TokenId` as `address`, use of `sendRequest` and `asyncCall`, and the importance of correctly encoding function signatures and contexts.

---

#### **1. TokenId as Address**

In **=nil;**, `TokenId` is an alias for the `address` type. When interacting with other contracts that require `TokenId` as an argument, you must treat `TokenId` as an `address` to ensure compatibility as Solidity only recognizes in-built types.

**Key Consideration:**

- **TokenId is treated as an `address`:** Always ensure that `TokenId` is passed as an `address` when encoding, decoding, or interacting with other contracts.

Example:

```solidity
function deposit() public payable {
    Nil.Token[] memory tokens = Nil.txnTokens();
    bytes memory callData = abi.encodeWithSignature(
        "recordDeposit(address,address,uint256)",
        msg.sender,
        tokens[0].id,  // TokenId is treated as address
        tokens[0].amount
    );
    Nil.asyncCall(globalLedger, address(this), 0, callData);
}
```

**Best Practice:**

- When working with `TokenId`, **always treat it as an `address`** in function signatures or contract calls.
- Ensure proper encoding of `TokenId` to prevent errors during cross contract execution.

---

#### **2. Using `sendRequest` and `sendRequestWithTokens`**

The **`sendRequest`** and **`sendRequestWithTokens`** functions are used to send asynchronous requests to other contracts. These functions are crucial for non-blocking operations, allowing the protocol to continue processing. When the request is processed at the destination contract, a response is sent and the function selector encoded in the `context` is invoked along with the response data.

Example of using `sendRequest`:

```solidity
bytes memory callData = abi.encodeWithSignature("getPrice(address)", borrowToken);
bytes memory context = abi.encodeWithSelector(
    this.processLoan.selector, msg.sender, amount, borrowToken, collateralToken
);
Nil.sendRequest(oracle, 0, 9_000_000, context, callData);
```

**Key Points:**

- **Function Selectors:** Always use `abi.encodeWithSelector` when creating function selectors to ensure the correct function is called during the callback.

```solidity
bytes memory context = abi.encodeWithSelector(
    this.processLoan.selector, msg.sender, amount, borrowToken, collateralToken
);
```

- **Signature Encoding:** The function signature must match exactly, including spaces, capitalization, and parameter types. Even small errors can lead to encoding failures.

```solidity
bytes memory callData = abi.encodeWithSignature("getPrice(address)", borrowToken);
```

**Best Practices:**

- **Ensure correct encoding of function signatures:** Be cautious about the spacing, capitalization, and parameter ordering. Even a small discrepancy will break the function call.
- Always verify that the `context` you create matches the structure expected by the callback function. Mismatches can cause decoding errors during execution.

---

#### **3. Handling Context and `asyncCall`**

The **`asyncCall`** function enables asynchronous interactions between contracts. In this example, `asyncCall` is used to record deposit information in the **GlobalLedger** contract while allowing the **LendingPool** contract to continue processing other transactions. The `asyncCall` doesnn't expect a callback and hence is a fire and forget function, where as `sendRequest` discussed above expects a callback.

Example of using `asyncCall` with context:

```solidity
bytes memory callData = abi.encodeWithSignature(
    "recordDeposit(address,address,uint256)",
    msg.sender,
    tokens[0].id,  // TokenId as address
    tokens[0].amount
);
Nil.asyncCall(globalLedger, address(this), 0, callData);
```

**Best Practices:**

- **Context Construction:** Use `abi.encodeWithSignature` or `abi.encode` to build the callData, ensuring it contains all the necessary data for the destination contract's function.

---

#### **4. Be Cautious with Space and Character Sensitivity**

Solidity is sensitive to even the smallest differences in function signatures, such as spaces or capitalization. This is crucial when working with ABI encoding functions like `abi.encodeWithSignature`.

**Example of Correct Encoding:**

```solidity
abi.encodeWithSignature("getPrice(address)", borrowToken)
```

**Example of Incorrect Encoding:**

```solidity
abi.encodeWithSignature("getPrice(address )", borrowToken)  // Incorrect due to space after address
```

**Best Practices:**

- **Verify the exact function signature:** Always check that the function signatures used in your contract calls match the exact expected format, including spaces, punctuation, and capitalization.

---

### **Conclusion**

This guide provides an example implementation of a decentralized lending and borrowing protocol on **=nil; Foundation**, using sharded smart contract architecture. **Please note that this is an educational example only and is not audited for production use.**

By following the coding practices outlined here, you can better understand how to:

1. Handle `TokenId` as an `address` when interacting with other contracts.
2. Correctly use `sendRequest` and `asyncCall` for asynchronous contract interactions.
3. Ensure that function signatures and contexts are encoded and decoded properly.
4. Avoid common pitfalls related to character sensitivity in ABI encoding.

While this example showcases how sharding and asynchronous communication can improve scalability and efficiency, remember that it is intended for learning and prototyping only. If you plan to deploy similar protocols in production, thorough auditing and testing are crucial to ensure security and reliability.
