import { waitTillCompleted } from "@nilfoundation/niljs";
import { bytesToHex } from "viem";
import type { HandlerContext } from "../context";
import { hexStringToUint8Array, shardNumber } from "../utils/conversion";
import { bigintReplacer } from "../utils/string";

export async function sendTransaction(params: any[], context: HandlerContext) {
  if (context.debug) {
    console.log(`Method eth_sendTransaction params ${JSON.stringify(params)}`);
  }
  if (params[0].to === undefined) {
    return prepareDeployment(params, context);
  }
  return handleDirectTransaction(params, context);
}

async function prepareDeployment(
  params: any[],
  context: HandlerContext,
): Promise<string> {
  const deployed = await context.wallet.deployContract({
    shardId:
      context.hre.config.shardId ?? shardNumber(context.wallet.getAddressHex()),
    bytecode: hexStringToUint8Array(params[0].data),
    args: [bytesToHex(context.wallet.pubkey)],
    salt: BigInt(Math.floor(Math.random() * 100000)),
    feeCredit: context.feeCredit,
  });
  if (context.debug) {
    console.log(`Response deployment ${JSON.stringify(deployed)}`);
  }

  const receipt = await waitTillCompleted(
    context.client,
    shardNumber(context.wallet.getAddressHex()),
    deployed.hash,
  );
  if (context.debug) {
    console.log(
      `Response deployment receipt ${JSON.stringify(receipt, bigintReplacer)}`,
    );
  }

  return receipt[0].outMessages?.[0] ?? "";
}

async function handleDirectTransaction(
  params: any[],
  context: HandlerContext,
): Promise<any> {
  const hash = await context.wallet.sendMessage({
    to: hexStringToUint8Array(params[0].to),
    feeCredit: context.directTxFeeCredit ?? 1000000n,
    value: context.directTxValue ?? 82309960n,
    data: hexStringToUint8Array(params[0].data),
  });
  if (context.debug) {
    console.log(`Response tx hash ${hash}`);
  }
  const receipt = await waitTillCompleted(
    context.client,
    shardNumber(context.wallet.getAddressHex()),
    hash,
  );
  if (context.debug) {
    console.log(
      `Response tx receipt ${JSON.stringify(receipt, bigintReplacer)}`,
    );
  }

  return hash;
}
