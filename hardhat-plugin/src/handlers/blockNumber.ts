import type { HandlerContext } from "../context";
import { executeOriginalFunction } from "../interceptors";
import { shardNumber } from "../utils/conversion";

export async function blockNumber(method: string, params: any[], context: HandlerContext) {
  const preparedMethod = "eth_getBlockByNumber";
  if (context.debug) {
    console.log("Method", preparedMethod);
  }
  const result = await executeOriginalFunction(preparedMethod, prepareInput(context), context);
  const adaptResponse = adaptResult(result);
  if (context.debug) {
    console.log("Response", JSON.stringify(adaptResponse));
  }
  return adaptResponse;
}

function prepareInput(context: HandlerContext): any[] {
  return [context.hre.config.shardId ?? shardNumber(context.wallet.address), "latest", false];
}

function adaptResult(result: any): any {
  if (!result) {
    return "0x0";
  }

  return result.number;
}
