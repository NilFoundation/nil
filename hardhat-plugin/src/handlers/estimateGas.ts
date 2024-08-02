import type { HandlerContext } from "../context";

export async function estimateGas(
  method: string,
  params: any[],
  context: HandlerContext,
) {
  if (context.debug) {
    console.log(`Method ${method} params ${JSON.stringify(params)}`);
    console.log("Response 0x0");
  }
  return "0x0";
}
