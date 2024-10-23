import type { HandlerContext } from "../context";
import { executeOriginalFunction } from "../interceptors";

export async function call(method: string, params: any[], context: HandlerContext) {
  const preparedParams = prepareInput(params);

  // For contract deployment skip call, as we have async message from wallet
  if (preparedParams[0] && preparedParams[0].to === undefined) {
    return "0x0";
  }

  if (context.debug) {
    console.log(`Method ${method} params ${JSON.stringify(preparedParams)}`);
  }
  const response = await executeOriginalFunction(method, preparedParams, context);
  if (context.debug) {
    console.log(`Response ${JSON.stringify(response)}`);
  }
  return response.data;
}

function prepareInput(params: any[]): any[] {
  if (params[0].to !== undefined) {
    if (params[0]) {
      params[0].feeCredit = "1000000000";
      params[0].from = null;
      params[0].value = 0;
    }
  }
  return params;
}
