import { HttpTransport, PublicClient } from "@nilfoundation/niljs";
import constants from "../constants";

/**
 * Creates and returns a public client instance to interact with the =nil; blockchain.
 * The client uses an HTTP transport and connects to a specific shard.
 *
 * @async
 * @returns {Promise<{publicClient: PublicClient}>} - A promise that resolves to a public client instance.
 * @throws {Error} If the RPC endpoint is not set in the environment variables.
 */
export async function createClient() {
  // Retrieve the RPC endpoint from constants
  const endpoint = constants.NIL_RPC_ENDPOINT;

  // Ensure the endpoint is defined before proceeding
  if (!endpoint) {
    throw new Error("RPC_ENDPOINT is not set in environment variables");
  }

  // Create a new PublicClient instance using the HTTP transport
  const publicClient = new PublicClient({
    transport: new HttpTransport({
      endpoint: endpoint, // Define the blockchain RPC URL for the transport layer
    }),
    shardId: 1, // Specify the shard ID (change if needed for a different shard)
  });

  return { publicClient }; // Return the created public client instance
}
