import {
  type FaucetClient,
  type Hex,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  SmartAccountV1,
  type Token,
  type Transaction,
  convertEthToWei,
  hexToBigInt,
  hexToBytes,
  toHex,
} from "@nilfoundation/niljs";
import { SmartAccount } from "@nilfoundation/smart-contracts";
import { encodeFunctionData } from "viem";
import { ActivityType } from "../../background/storage";
import { TokenNames } from "../components/token";
import { addActivity } from "../store/model/activities.ts";
import { generateRandomSalt } from "../utils";
import { topUpSpecificToken } from "./faucet.ts";

// Create Public Client
export function createClient(rpcEndpoint: string, shardId: number): PublicClient {
  const appVersion = import.meta.env.VITE_APP_VERSION || "1.0";

  return new PublicClient({
    transport: new HttpTransport({
      endpoint: rpcEndpoint,
      headers: {
        "Client-Type": `wallet v${appVersion}`,
      },
    }),
    shardId,
  });
}

// Create Signer
export function createSigner(privateKey: Hex): LocalECDSAKeySigner {
  return new LocalECDSAKeySigner({ privateKey });
}

// Deploy a brand-new smartAccount or re-init an existing one
export async function initializeOrDeploySmartAccount(params: {
  client: PublicClient;
  signer: LocalECDSAKeySigner;
  faucetClient: FaucetClient;
  shardId: number;
  existingSmartAccountAddress?: Hex;
}): Promise<SmartAccountV1> {
  const { client, signer, shardId, existingSmartAccountAddress, faucetClient } = params;
  const pubkey = signer.getPublicKey();

  try {
    // If we already have a smartAccount address, re-init it
    if (existingSmartAccountAddress) {
      console.log("Initializing smart account with existing address:", existingSmartAccountAddress);
      return new SmartAccountV1({
        pubkey,
        address: hexToBytes(existingSmartAccountAddress as Hex),
        client,
        signer,
      });
    }

    // Otherwise, deploy a new smartAccount
    console.log("Deploying a new smart account...");
    const smartAccount = new SmartAccountV1({
      pubkey,
      salt: generateRandomSalt(),
      shardId: shardId,
      client,
      signer,
    });

    try {
      // Top up smartAccount with 0.1 native token
      await topUpSpecificToken(
        smartAccount,
        faucetClient,
        TokenNames.NIL,
        convertEthToWei(0.009),
        false,
      );
    } catch (e) {
      console.error("Failed to top up smartAccount during deployment:", e);
      throw new Error("Failed to top up smartAccount");
    }

    try {
      // Deploy the smartAccount
      await smartAccount.selfDeploy(true);
      console.log("SmartAccount deployed successfully at:", smartAccount.address);
    } catch (e) {
      console.error("Failed to self-deploy the smartAccount:", e);
      throw new Error("Failed to self-deploy smartAccount");
    }

    return smartAccount;
  } catch (e) {
    console.error("Error during smartAccount initialization or deployment:", e);
    throw new Error("SmartAccount initialization or deployment failed");
  }
}

// Send token
export async function sendToken({
  smartAccount,
  to,
  value,
  tokenAddress,
  symbol,
}: {
  smartAccount: SmartAccountV1;
  to: Hex;
  value: number;
  tokenAddress: string;
  symbol: string;
}): Promise<void> {
  let tx: Transaction | null = null;
  const feeCredit = 100_000_000_000_000n * 10n;

  try {
    // Determine transaction parameters
    const transactionParams =
      tokenAddress === ""
        ? getNilTransactionParams(to, value, feeCredit)
        : getTokenTransactionParams(to, value, tokenAddress, feeCredit);

    // Send transaction
    tx = await smartAccount.sendTransaction(transactionParams);
    console.log(`Transaction sent for ${tokenAddress}, hash: ${tx.hash}`);

    // Wait for transaction to complete
    const receipt = await tx.wait();
    if (!receipt[0].success) {
      throw new Error("Transaction failed");
    }
    console.log("Transaction completed:", receipt);

    // Log successful transaction
    logActivity(smartAccount.address, tx.hash, true, value, symbol);
  } catch (e) {
    if (tx) {
      logActivity(smartAccount.address, tx.hash, false, value, symbol);
    }
    console.log("Failed to send token:", e);
    throw new Error(`Failed to send ${value} ${tokenAddress} to ${to}`);
  }
}

export async function sendTransaction({
  smartAccount,
  transactionParams,
}: {
  smartAccount: SmartAccountV1;
  transactionParams: {
    to: string;
    value?: number;
    tokens?: { id: string; amount: bigint }[];
    data?: string | null;
    feeCredit?: bigint;
  };
}): Promise<string> {
  const feeCredit = 100_000_000_000_000n * 10n;

  // Convert value to bigint if it's defined and a number
  const valueInWei =
    transactionParams.value !== undefined ? convertEthToWei(transactionParams.value) : 0n;
  try {
    // Ensure feeCredit is always included
    const txParams = {
      ...transactionParams,
      value: valueInWei,
      feeCredit,
    };

    const tx = await smartAccount.sendTransaction(txParams);

    return tx.hash;
  } catch (e) {
    throw new Error("Failed to send transaction");
  }
}

// Get transaction parameters for NIL (native currency)
function getNilTransactionParams(to: Hex, value: number, feeCredit: bigint) {
  return {
    to,
    value: convertEthToWei(value),
    feeCredit,
  };
}

// Get transaction parameters for token transfers
function getTokenTransactionParams(
  to: Hex,
  value: number,
  tokenAddress: string,
  feeCredit: bigint,
) {
  return {
    to,
    value: 0n,
    feeCredit,
    tokens: [
      {
        id: tokenAddress as Hex,
        amount: hexToBigInt(toHex(value)),
      },
    ],
  };
}

// Log transaction activity
function logActivity(
  smartAccountAddress: Hex,
  txHash: Hex,
  success: boolean,
  amount: number,
  token: string,
) {
  addActivity({
    smartAccountAddress: smartAccountAddress,
    activity: {
      activityType: ActivityType.SEND,
      txHash,
      success,
      amount: amount.toString(),
      token,
    },
  });
}

export async function estimateFee(
  smartAccount: SmartAccountV1,
  to: Hex,
  value: bigint,
  tokens: Token[],
): Promise<bigint> {
  try {
    if (!smartAccount?.client) {
      throw new Error(
        "SmartAccount client is unavailable. Ensure the smartAccount is initialized.",
      );
    }

    // Encode `asyncCall` function data for gas estimation
    const callData = encodeFunctionData({
      abi: SmartAccount.abi,
      functionName: "asyncCall",
      args: [
        to,
        smartAccount.address,
        smartAccount.address,
        tokens,
        BigInt(value), // Convert value to BigInt
        "0x",
      ],
    });

    // Estimate gas cost
    const { feeCredit } = await smartAccount.client.estimateGas(
      {
        to: smartAccount.address,
        from: smartAccount.address,
        data: hexToBytes(callData),
      },
      "latest",
    );
    console.log("feeCredit: ", feeCredit);
    return feeCredit;
  } catch (error) {
    console.error("Failed to estimate gas fee:", error);
    throw new Error(error.message);
  }
}
