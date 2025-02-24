import { FaucetClient, type Hex, HttpTransport, type SmartAccountV1 } from "@nilfoundation/niljs";
import { ActivityType } from "../../background/storage";
import { Currency } from "../components/currency";
import { addActivity } from "../store/model/activities.ts";
import { convertWeiToEth } from "../utils";
import { fetchBalance } from "./balance.ts";

// Create Faucet
export function createFaucetClient(rpcEndpoint: string): FaucetClient {
  const appVersion = import.meta.env.VITE_APP_VERSION || "1.0";

  return new FaucetClient({
    transport: new HttpTransport({
      endpoint: rpcEndpoint,
      headers: {
        "Client-Type": `wallet v${appVersion}`,
      },
    }),
  });
}

// Top up the smartAccount with *all* tokens from every available faucet
export async function topUpAllCurrencies(
  smartAccount: SmartAccountV1,
  faucetClient: FaucetClient,
  amount = 10n,
): Promise<void> {
  try {
    // Get the map of all faucets: { [tokenNameOrAddr]: faucetAddress }
    const faucets = await faucetClient.getAllFaucets();
    if (!faucets || Object.keys(faucets).length === 0) {
      throw new Error("No faucets available for top-up");
    }

    const faucetAddresses = Object.values(faucets);
    console.log("Topping up tokens for each faucet:", faucetAddresses);

    // Create promises for each faucet top-up
    const promises = faucetAddresses.map((faucetAddress) =>
      faucetClient.topUpAndWaitUntilCompletion(
        {
          smartAccountAddress: smartAccount.address,
          faucetAddress,
          amount,
        },
        smartAccount.client,
      ),
    );

    // Await all promises to ensure completion or capture errors
    await Promise.all(promises);
    console.log("Successfully topped up all currencies");
  } catch (e) {
    // Log the overall error and rethrow it
    console.error("Error during top-up of all currencies:", e);
    throw new Error("Failed to top up all currencies");
  }
}

// TopUp smartAccount with specific currency
export async function topUpSpecificCurrency(
  smartAccount: SmartAccountV1,
  faucetClient: FaucetClient,
  symbol: string,
  amount: bigint,
  showInActivity = true,
): Promise<void> {
  console.log(`Topping up ${amount} ${symbol} to smartAccount ${smartAccount.address}...`);

  const faucetAddress = await getFaucetAddress(faucetClient, symbol);
  let txHash: string | null = null;

  let initialBalance: bigint | null = null;
  if (symbol === Currency.NIL) {
    initialBalance = await fetchBalance(smartAccount);
  }

  try {
    // Perform the top-up
    txHash = await faucetClient.topUpAndWaitUntilCompletion(
      {
        smartAccountAddress: smartAccount.address,
        faucetAddress,
        amount,
      },
      smartAccount.client,
    );

    // Verify transaction receipt
    const receipt = await smartAccount.client.getTransactionReceiptByHash(txHash as Hex);
    if (!receipt?.success) {
      throw new Error("Top up message failed");
    }

    console.log(
      `Successfully topped up ${amount} ${symbol} to smartAccount ${smartAccount.address}, txHash: ${txHash}`,
    );

    let actualReceived: string = amount.toString();

    // Compute actual received amount if NIL
    if (symbol === Currency.NIL && initialBalance !== null) {
      const finalBalance = await fetchBalance(smartAccount);
      actualReceived = convertWeiToEth(finalBalance - initialBalance, 11);
      console.log(
        `Balance before: ${convertWeiToEth(initialBalance)}, after: ${convertWeiToEth(finalBalance)}`,
      );
    }

    // Log success activity
    if (showInActivity) {
      logTopUpActivity(smartAccount.address, txHash, true, actualReceived, symbol);
    }
  } catch (e) {
    console.error(`Error during ${symbol} top-up:`, e);

    // Log failure
    if (txHash && showInActivity) {
      logTopUpActivity(smartAccount.address, txHash, false, amount.toString(), symbol);
    }

    throw new Error(`Failed to top up ${symbol}`);
  }
}

// Get faucet address by token symbol
async function getFaucetAddress(faucetClient: FaucetClient, symbol: string): Promise<Hex> {
  const faucets = await faucetClient.getAllFaucets();
  if (!faucets || !faucets[symbol.toUpperCase()]) {
    throw new Error(`No faucet available for ${symbol}`);
  }
  return faucets[symbol.toUpperCase()];
}

function logTopUpActivity(
  smartAccountAddress: Hex,
  txHash: string,
  success: boolean,
  amount: string,
  symbol: string,
): void {
  addActivity({
    smartAccountAddress: smartAccountAddress,
    activity: {
      activityType: ActivityType.TOPUP,
      txHash,
      success,
      amount: amount.toString(),
      token: symbol,
    },
  });
}
