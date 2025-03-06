import { HttpTransport, PublicClient } from "@nilfoundation/niljs";
import { createDomain } from "effector";

export const shardsAmountDomain = createDomain("shardsAnount");

export const $shardsAmount = shardsAmountDomain.createStore<number>(-1);
export const getShardsAmountFx = shardsAmountDomain.createEffect<string, number>();

getShardsAmountFx.use(async (rpcUrl) => {
  const client = new PublicClient({
    transport: new HttpTransport({ endpoint: rpcUrl }),
  });
  return (await client.getShardIdList()).length;
});
