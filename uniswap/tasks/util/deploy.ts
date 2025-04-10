import { type SmartAccountV1, getContract } from "@nilfoundation/niljs";
import type { Abi } from "abitype";

export async function deployNilContract(
  smartAccount: SmartAccountV1,
  abi: Abi,
  bytecode: string,
  args: unknown[] = [],
  shardId?: number,
  externalMethods: string[] = [],
) {
  const { tx, address } = await smartAccount.deployContract({
    abi: abi,
    args: args,
    // @ts-ignore
    bytecode: `${bytecode}`,
    salt: BigInt(Math.floor(Math.random() * 1000000)),
    shardId: shardId ?? smartAccount.shardId,
    feeCredit: BigInt("19340180000000"),
  });

  const receipts = await tx.wait();
  if (!receipts.every((receipt) => receipt.success)) {
    throw new Error(
      `One or more receipts indicate failure: ${JSON.stringify(receipts)}`,
    );
  }
  console.log("Contract deployed at address: " + address);

  const contract = getContract({
    abi: abi,
    address: address,
    client: smartAccount.client,
    smartAccount: smartAccount,
    externalInterface: {
      signer: smartAccount.signer,
      methods: externalMethods,
    },
  });

  const code = await smartAccount.client.getCode(address);
  if (!code) {
    throw new Error(
      "No code for deployed contract " + address + ", hash: " + tx.hash,
    );
  }

  return { contract, address };
}
