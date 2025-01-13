import {
  type WalletV1,
  getContract,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import type { Abi } from "abitype";

export async function deployNilContract(
  wallet: WalletV1,
  abi: Abi,
  bytecode: string,
  args: unknown[] = [],
  shardId?: number,
  externalMethods: string[] = [],
) {
  const { hash, address } = await wallet.deployContract({
    abi: abi,
    args: args,
    // @ts-ignore
    bytecode: `${bytecode}`,
    salt: BigInt(Math.floor(Math.random() * 1000000)),
    shardId: shardId ?? wallet.shardId,
  });

  const receipts = await waitTillCompleted(wallet.client, hash);
  if (!receipts.every((receipt) => receipt.success)) {
    throw new Error(
      `One or more receipts indicate failure: ${JSON.stringify(receipts)}`,
    );
  }
  console.log("Contract deployed at address: " + address);

  const contract = getContract({
    abi: abi,
    address: address,
    client: wallet.client,
    wallet: wallet,
    externalInterface: {
      signer: wallet.signer,
      methods: externalMethods,
    },
  });

  const code = await wallet.client.getCode(address);
  if (!code) {
    throw new Error(
      "No code for deployed contract " + address + ", hash: " + hash,
    );
  }

  return { contract, address };
}
