import type { HandlerContext } from "./context";
import { blockNumber } from "./handlers/blockNumber";
import { call } from "./handlers/call";
import { chainId } from "./handlers/chainId";
import { estimateGas } from "./handlers/estimateGas";
import { gasPrice } from "./handlers/gasPrice";
import { getBlockByNumber } from "./handlers/getBlockByNumber";
import { getTransactionByHash } from "./handlers/getTransactionByHash";
import { getTransactionCount } from "./handlers/getTransactionCount";
import { getTransactionReceipt } from "./handlers/getTransactionReceipt";
import { sendTransaction } from "./handlers/sendTransaction";

export async function unifiedInterceptor(
  method: string,
  params: any[],
  context: HandlerContext,
) {
  switch (method) {
    case "eth_getTransactionReceipt":
      return getTransactionReceipt(method, params, context);
    case "eth_getTransactionByHash":
      return getTransactionByHash(method, params, context);
    case "eth_getTransactionCount":
      return getTransactionCount(method, params, context);
    case "eth_sendTransaction":
      return sendTransaction(params, context);
    case "eth_getBlockByNumber":
      return getBlockByNumber(method, params, context);
    case "eth_blockNumber":
      return blockNumber(method, params, context);
    case "eth_estimateGas":
      return estimateGas(method, params, context);
    case "eth_gasPrice":
      return gasPrice(method, params, context);
    case "eth_call":
      return call(method, params, context);
    case "eth_chainId":
      return chainId(method, params, context);
    default:
      if (context.isRequest) {
        return await context.originalRequest({
          method: method,
          params: params,
        });
      }
      return await context.originalSend(method, params);
  }
}

export async function executeOriginalFunction(
  method: string,
  params: any[],
  context: HandlerContext,
) {
  if (context.isRequest) {
    return await context.originalRequest({ method, params });
  }
  return await context.originalSend(method, params);
}
