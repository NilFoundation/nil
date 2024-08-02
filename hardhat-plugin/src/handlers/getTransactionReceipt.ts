import type { HandlerContext } from "../context";
import { executeOriginalFunction } from "../interceptors";
import { shardNumber } from "../utils/conversion";

export async function getTransactionReceipt(
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
  const response = adaptResponse(result, preparedParams);
  if (context.debug) {
    console.log(`Response ${JSON.stringify(response)}`);
  }
  return response;
}

function prepareInput(
  method: string,
  params: any[],
  context: HandlerContext,
): [string, any[]] {
  return [
    "eth_getInMessageReceipt",
    [
      context.hre.config.shardId ?? shardNumber(context.wallet.getAddressHex()),
      ...params,
    ],
  ];
}

function adaptResponse(result: any, params: any[]): any {
  if (!result) {
    return null;
  }

  result.transactionIndex = 1;
  result.blockNumber = String(result.blockNumber);
  result.status =
    typeof result.success === "boolean"
      ? result.success
        ? "0x1"
        : "0x0"
      : result.success;
  result.contractAddress = result.contractAddress
    ? String(result.contractAddress)
    : null;
  result.gasUsed = String(result.gasUsed);
  result.logs = [];
  result.index = 1;
  result.hash = String(params[1]);
  result.transactionHash = String(params[1]);
  result.cumulativeGasUsed = result.gasUsed;
  return result;
}
