# <p align="center">=nil; Wallet Extension 🔐</p>

Welcome to the **=nil; Wallet Extension** — a user-friendly way to manage accounts on the **=nil;** blockchain. This extension provides essential tools for interacting with the network or connecting with dApps. Give it a spin and let us know what you think! 🚀

---

## Roadmap 🛣️

- [ ] **Basic Wallet Features** ✅
    - Create a new wallet
    - Send & Receive tokens
    - Check your balance
    - Top-up test currencies
- [ ] **Wallet SDK for dApp Developers** ✅
- [ ] **Playground Integration**
- [ ] **Further Improvements & Feature Requests**

---

## Languages 🌐

By default, everything is in **English**. However, we plan to support multiple languages in the near future. We use **i18next** to handle translations, and we'd love to hear which languages you'd like to see!

---

## Wallet SDK 🚀

The =nil; Wallet Extension comes with a built-in SDK that follows the [EIP-1193 Standard](https://eips.ethereum.org/EIPS/eip-1193)

We are also actively working on supporting the [EIP-6963 Standard](https://eips.ethereum.org/EIPS/eip-6963) to further improve multi-provider support.

The extension injects the `window.nil` object into the browser. Currently, we support the following two methods:

1. **eth_sendTransaction:** Send transactions directly from the wallet.
2. **eth_requestAccounts:** Request user wallet addresses.

To learn more about how to use the SDK, visit the [SDK Documentation](#)

---

## How to Use ⚙️

1. **Clone** this repository to your local machine
2. Install dependencies:
   ```sh
   npm install
   ```
3. Build the `smart contracts`:
   ```sh
   cd smart-contracts && npm run build && cd ..
   ```
4. Build `niljs`:
   ```sh
   cd niljs && npm run build && cd ..
   ```
5. Build the `wallet extension`:
   ```sh
   cd wallet-extension && npm run build
   ```
   This generates the production files.
6. Load the extension in Chrome:
  - Open **Chrome Extensions** (`chrome://extensions/`)
  - Enable **Developer Mode**
  - Load the generated folder as an **unpacked extension**

That’s it! Enjoy managing your tokens and exploring dApps on the **=nil;** blockchain. If you have questions or suggestions, feel free to open an issue or reach out. 💡

## Contribution 🤝

We welcome contributions from the community! To view all issues, visit the [GitHub Issues page](https://github.com/NilFoundation/nil/issues?q=is%3Aissue%20state%3Aopen%20label%3A%22wallet%20extension%22) and filter by the `wallet extension` label.

We also use the [good first issue](https://github.com/NilFoundation/nil/issues?q=is%3Aissue%20state%3Aopen%20label%3A%22good%20first%20issue%22%20label%3A%22wallet%20extension%22) label for tasks that are beginner-friendly and open to outside contributors.

If you find a bug or want to request a new feature, please open a new issue and fill out the following template:

### Issue Template 📝

```sh
## Description: 
// Briefly describe the issue or feature request

## Acceptance Criteria:
// Outline what must be true for the issue to be considered complete

## Technical Notes:
// Add any technical details or implementation considerations

## Figma/Design:
// Link to any relevant designs or wireframes
```