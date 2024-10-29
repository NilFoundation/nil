import type { HandlerContext } from "../context";
import { executeOriginalFunction } from "../interceptors";
import { shardNumber } from "../utils/conversion";

export async function getTransactionCount(method: string, params: any[], context: HandlerContext) {
  const preparedParams = await prepareInput(params, context);
  if (context.debug) {
    console.log(`Method ${method} params ${JSON.stringify(preparedParams)}`);
  }
  const result = await executeOriginalFunction(method, preparedParams, context);
  if (context.debug) {
    console.log(`Response params ${JSON.stringify(result)}`);
  }
  return result;
}

async function prepareInput(params: any[], context: HandlerContext): Promise<any[]> {
  const address = await context.signer.getAddress(
    context.hre.config.shardId ?? shardNumber(context.wallet.address),
  ); // Simulate async call if necessary
  return [`0x${Buffer.from(address).toString("hex")}`, "latest"];
}
