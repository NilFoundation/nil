
# Transaction Types

The `=nil; Wallet Extension` supports the following transaction types:
1. **Send Native Token**
2. **Send Tokens from Wallet**
3. **Interact with Any Contract**

**Note:** Contract deployment is currently not supported, but we plan to add this feature in the near future.

---

## 1. Sending Native Token

To send native tokens, construct the transaction object as follows:

```javascript
const tx = {
    to: "0x000150ca877f809d7095871b791858ad2c9c4372",
    value: 0.001,
};

try {
    const txHash = await window.nil.request({
        method: "eth_sendTransaction",
        params: [tx],
    });
    console.log(`✅ Transaction sent: ${txHash}`);
} catch (error) {
    console.error("❌ Failed to send transaction:", error);
}
```

### Important Notes:
- **Value Type:** Ensure the `value` is a `number` and greater than `0`
- **BigInt Not Supported:** Passing `BigInt` will fail due to Chrome port limitations.
  Example of unsupported usage:
  ```javascript
  value: 3n;
  ```
  This will prevent the modal from opening

---

## 2. Sending Tokens

To send tokens from the wallet, use the `tokens` attribute:

```javascript
const tx = {
    to: "0x000150ca877f809d7095871b791858ad2c9c4372",
    tokens: [{ id: "0x0001111111111111111111111111111111111114", amount: 1 }]
};

try {
    const txHash = await window.nil.request({
        method: "eth_sendTransaction",
        params: [tx],
    });
    console.log(`✅ Transaction sent: ${txHash}`);
} catch (error) {
    console.error("❌ Failed to send transaction:", error);
}
```

### Token Structure:
- **id:** Token address
- **amount:** Number of tokens (must be an integer greater than `0`)

Example Structure:
```javascript
[{ id: "0x0001111111111111111111111111111111111114", amount: 1 }]
```

---

## 3. Contract Interaction

To interact with a smart contract, encode the transaction data using `viem`:

1. **Install viem if not already installed:**
```bash
npm install viem
```

2. **Import and Encode Transaction:**
```javascript
import { encodeFunctionData } from "viem";

const data = encodeFunctionData({
    abi: CONTRACT_ABI,
    functionName: "increment",
    args: [],
});
```

3. **Construct and Send Transaction:**
```javascript
const tx = {
    to: CONTRACT_ADDRESS,
    data: data
};

try {
    const txHash = await window.nil.request({
        method: "eth_sendTransaction",
        params: [tx],
    });
    console.log(`✅ Contract interaction successful: ${txHash}`);
} catch (error) {
    console.error("❌ Failed to interact with contract:", error);
}
```

### Learn More:
For more details on encoding transactions, refer to the [=nil; documentation](https://docs.nil.foundation/nil/niljs/reading-writing-info).

---

## Combining Transaction Types

These transaction attributes are **not mutually exclusive**. You can combine them as needed, such as sending native tokens and tokens together:

```javascript
const tx = {
    to: "0x000150ca877f809d7095871b791858ad2c9c4372",
    value: 0.001,
    tokens: [{ id: "0x0001111111111111111111111111111111111114", amount: 1 }]
};

try {
    const txHash = await window.nil.request({
        method: "eth_sendTransaction",
        params: [tx],
    });
    console.log(`✅ Combined transaction sent: ${txHash}`);
} catch (error) {
    console.error("❌ Failed to send combined transaction:", error);
}
```

---

## Error Handling

For a comprehensive list of potential errors and their resolutions, check out the [errors.md](./errors.md) file.
