
# SDK Error Structure

This document outlines the structure of errors in the =nil; Wallet SDK. Errors are designed to follow the [EIP-1193](https://eips.ethereum.org/EIPS/eip-1193) standard, ensuring consistency across dApp interactions

## Error Codes and Messages

The SDK uses specific error codes and corresponding messages to describe issues encountered during requests. Below is a breakdown of the error codes and what they represent:

### Error Codes

| **Error Code**      | **Error Name**          | **Description**                                    |
|---------------------|--------------------------|----------------------------------------------------|
| `32602`             | `INVALID_PARAMS`         | Parameters provided are invalid or malformed       |
| `4001`              | `USER_REJECTED`          | User rejected the request in the wallet modal      |
| `4100`              | `UNAUTHORIZED`           | The dApp is not authorized to access the user's account |
| `4200`              | `UNSUPPORTED_METHOD`     | The requested method is not supported by the wallet |
| `4900`              | `PROVIDER_DISCONNECTED`  | The wallet provider is disconnected                |
| `4901`              | `CHAIN_DISCONNECTED`     | The blockchain network is disconnected             |
| `-32603`            | `INTERNAL_ERROR`         | An internal error occurred during the request      |

---

### Error Messages

Each error code is associated with a specific message to clarify the issue. Here are the common error messages used in the SDK:

- **MISSING_ORIGIN**: `Missing origin URL` – The request lacks the required origin field
- **USER_REJECTED**: `User rejected the request` – The user declined the request in the wallet modal
- **UNAUTHORIZED**: `Unauthorized: Account is not connected` – The dApp is not connected to the wallet
- **INVALID_SMART_ACCOUNT**: `Invalid smartAccount address` – The provided smart account address is invalid
- **INVALID_VALUE**: `Invalid value. Please enter a valid number` – The `value` field must be a valid number
- **VALUE_TOO_LOW**: `Transaction value must be greater than zero` – The transaction amount must be positive
- **INVALID_TOKEN_ARRAY**: `Tokens must be an array` – The `tokens` field must be an array of objects
- **MISSING_PARAMS**: `'params' field is required and must be a single-item array` – Invalid request structure
- **INVALID_TO_FIELD**: `'to' field is required and must be a valid string` – The `to` field must be a valid address
- **MISSING_TRANSACTION_FIELDS**: `At least one of 'value', 'tokens', or 'data' must be provided` – Transaction fields are missing
- **INVALID_TOKEN_ID**: `Invalid token ID: {id}` – The provided token ID is invalid
- **INVALID_TOKEN_AMOUNT**: `Invalid token amount for ID {id}` – Token amount must be a positive integer
- **DECIMAL_TOKEN_AMOUNT**: `Token amount for {id} cannot be a decimal value` – Token amounts must be whole numbers
- **UNSUPPORTED_METHOD**: `Unsupported method: {method}` – The requested method is not supported by the SDK

---

## Example of Error Handling

Here's an example of how to handle errors when interacting with the wallet extension:

```javascript
try {
    const txHash = await window.nil.request({
        method: "eth_sendTransaction",
        params: [tx],
    });
    console.log(`Transaction sent: ${txHash}`);
} catch (error) {
    console.error(`Failed to send transaction: ${error.message} (Code: ${error.code})`);
}
```

---

For more details, refer to the [official EIP-1193 documentation](https://eips.ethereum.org/EIPS/eip-1193).
