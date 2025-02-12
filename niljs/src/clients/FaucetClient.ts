import { toHex } from "../encoding/toHex.js";
import type { Hex } from "../types/Hex.js";
import { waitTillCompleted } from "../utils/receipt.js";
import { BaseClient } from "./BaseClient.js";
import type { PublicClient } from "./PublicClient.js";
import type { FaucetClientConfig } from "./types/Configs.js";

/**
 * The parameters for the top up request.
 */
export type TopUpParams = {
  faucetAddress: Hex;
  smartAccountAddress: Hex;
  amount: bigint;
};

/**
 * FaucetClient is a client that interacts with the faucet api.
 * It is used to get information about the faucet and to top up the account with custom tokens.
 * @class FaucetClient
 * @extends BaseClient
 * @example
 * import { FaucetClient } from '@nilfoundation/niljs';
 *
 * const faucetClient = new FaucetClient({
 *   transport: new HttpTransport({
 *     endpoint: FAUCET_ENDPOINT,
 *   }),
 * });
 */
class FaucetClient extends BaseClient {
  // biome-ignore lint/complexity/noUselessConstructor: may be useful in the future
  constructor(config: FaucetClientConfig) {
    super(config);
  }

  /**
   * Gets all the faucets available.
   * @returns The list of all faucets. The key is the token name and the value is faucet address.
   */
  public async getAllFaucets() {
    return await this.request<Record<string, Hex>>({
      method: "faucet_getFaucets",
      params: [],
    });
  }

  /**
   * Topups the smart account with the specified amount of token which can be issued by the faucet.
   * @param param - The parameters for the top up request.
   * @param param.smartAccountAddress - The address of the smart account to top up.
   * @param param.faucetAddress - The address of the faucet to use.
   * @param param.amount - The amount to top up.
   * @returns The transaction hash of the top up transaction.
   */
  public async topUp({ faucetAddress, smartAccountAddress, amount }: TopUpParams) {
    return await this.request<Hex>({
      method: "faucet_topUpViaFaucet",
      params: [faucetAddress, smartAccountAddress, toHex(amount)],
    });
  }

  /**
   * Topups the smart account with the specified amount of token which can be issued by the faucet.
   * This function waits until the top up transaction is completed.
   * @param param - The parameters for the top up request.
   * @param param.smartAccountAddress - The address of the smart account to top up.
   * @param param.faucetAddress - The address of the faucet to use.
   * @param param.amount - The amount to top up.
   * @param client - Public client to fetch the data from the network.
   * @param [retries] - The number of retries to make.
   * @returns The transaction hash of the top up transaction.
   */
  public async topUpAndWaitUntilCompletion(
    { smartAccountAddress, faucetAddress, amount }: TopUpParams,
    client: PublicClient,
    retries = 5,
  ) {
    let currentRetry = 0;
    while (currentRetry++ < retries) {
      try {
        const txHash = await this.topUp({
          faucetAddress,
          smartAccountAddress,
          amount,
        });

        const receipts = await Promise.race([
          new Promise<[]>((resolve) => setTimeout(() => resolve([]), 10000)),
          waitTillCompleted(client, txHash),
        ]);

        if (receipts.length === 0) {
          continue;
        }

        if (receipts.some((receipt) => !receipt.success)) {
          continue;
        }

        return txHash;
      } catch (error) {
        await new Promise((resolve) => setTimeout(resolve, 1000));

        if (currentRetry === retries) {
          throw error;
        }
      }
    }

    throw new Error("Failed to withdraw to the given address");
  }
}

export { FaucetClient };
