import {
  type WalletV1,
  getContract,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import type { Address } from "abitype";
import CurrencyJson from "../../artifacts/contracts/Currency.sol/Currency.json";

/**
 * Function to mint and send currency from a contract.
 */
export async function mintAndSendCurrency({
  wallet,
  contractAddress,
  walletAddress,
  mintAmount,
}: MintAndSendCurrencyArgs) {
  const contract = getContract({
    abi: CurrencyJson.abi,
    address: contractAddress,
    client: wallet.client,
    wallet: wallet,
    externalInterface: {
      signer: wallet.signer,
      methods: ["mintCurrency", "sendCurrency"],
    },
  });

  const hash1 = await contract.external.mintCurrency([mintAmount]);
  await waitTillCompleted(wallet.client, hash1);
  await sleep(3000);
  const hash2 = await contract.external.sendCurrency([
    walletAddress,
    contractAddress,
    mintAmount,
  ]);
  await waitTillCompleted(wallet.client, hash2);
}

export function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export interface MintAndSendCurrencyArgs {
  wallet: WalletV1;
  contractAddress: Address;
  walletAddress: Address;
  mintAmount: bigint;
}
