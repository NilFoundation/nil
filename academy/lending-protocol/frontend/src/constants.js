/**
 * Configuration constants for the frontend application.
 * These values are used throughout the app for interacting with the =nil; blockchain.
 */

// Define the RPC endpoint for connecting to the =nil; blockchain
// Replace "your_rpc_endpoint" with the actual RPC URL for your network
const NIL_RPC_ENDPOINT = "your_rpc_endpoint";

// Token contract addresses
// These addresses correspond to the ETH and USDT tokens on the =nil; blockchain
const ETH = "0x0001111111111111111111111111111111111112";
const USDT = "0x0001111111111111111111111111111111111113";

// Address of the deployed lending pool contract
// This should be updated with the actual deployed contract address after deployment
const contractAddress = "your_contract_address";

// Export constants for use throughout the application
export default { NIL_RPC_ENDPOINT, ETH, USDT, contractAddress };
