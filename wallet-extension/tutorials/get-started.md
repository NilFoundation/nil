
# Getting Started with =nil; Wallet Extension 🚀

The =nil; Wallet Extension exposes `window.nil` as a provider compatible with [EIP-1193](https://eips.ethereum.org/EIPS/eip-1193). This allows decentralized applications (dApps) to interact with the =nil; blockchain securely.

---

## 📥 Connecting Your dApp

Before interacting with the wallet, ensure your dApp is connected:

```javascript
const accounts = await window.nil.request({ method: "eth_requestAccounts" });
```

If connected, this returns an array of accounts, with the first address accessible via:

```javascript
const userAddress = accounts[0];
```

The extension tracks connected dApps. To view or remove connections, go to:

- **=nil; Extension → Settings → Manage Dapps**

Once connected, future calls like this:

```javascript
const accounts = await window.nil.request({ method: "eth_requestAccounts" });
```

will immediately return the connected smart account without reopening the modal.

---

## 💸 Sending Transactions

Once connected, you can send transactions without exposing private keys. Supported actions include:

1. Sending native tokens
2. Transferring tokens
3. Smart contract interactions

Example of sending native tokens:

```javascript
const tx = {
    to: "0x000150ca877f809d7095871b791858ad2c9c4372",
    value: 0.001,
};

const txHash = await window.nil.request({
    method: "eth_sendTransaction",
    params: [tx],
});

console.log("Transaction Hash:", txHash);
```

👉 **Note:** For read-only contract interactions, you can directly use `nil.js` without the extension.

---

## 📚 Learn More

To explore how to send different types of transactions, check [transaction-types.md](./transaction-types.md).

---

That's it! You're now ready to connect your dApp and start interacting with the =nil; blockchain. 🎉
