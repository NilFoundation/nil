import type { HandlerContext } from "../context";
import { executeOriginalFunction } from "../interceptors";
import { shardNumber } from "../utils/conversion";

export async function getTransactionByHash(
  method: string,
  params: any[],
  context: HandlerContext,
) {
  const [preparedMethod, preparedParams] = prepareInput(
    method,
    params,
    context,
  );
  if (context.debug) {
    console.log(
      `Method ${preparedMethod} params ${JSON.stringify(preparedParams)}`,
    );
  }
  const result = await executeOriginalFunction(
    preparedMethod,
    preparedParams,
    context,
  );
  const adaptResponse = adaptResult(result);
  if (context.debug) {
    console.log(`Response ${JSON.stringify(adaptResponse)}`);
  }
  return adaptResponse;
}

function prepareInput(
  method: string,
  params: any[],
  context: HandlerContext,
): [string, any[]] {
  const preparedMethod = "eth_getInMessageByHash";
  const preparedParams = [
    context.hre.config.shardId ?? shardNumber(context.wallet.getAddressHex()),
    ...params,
  ];
  return [preparedMethod, preparedParams];
}

function adaptResult(result: any): any {
  if (!result) {
    return result;
  }

  result.gasPrice = "10";
  result.gas = result.gasUsed;

  if (result.signature === "0x") {
    // Hardhat wants a signature, so we'll give it a fake one.
    result.signature = `0x${"00".repeat(64)}`;
  }

  result.nonce = result.seqno;
  return result;
}
