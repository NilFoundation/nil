import { FaucetClient } from "../clients/FaucetClient.js";
import { PublicClient } from "../clients/PublicClient.js";
import { HttpTransport } from "../transport/HttpTransport.js";
import type { Hex } from "../types/Hex.js";
import { getShardIdFromAddress } from "./address.js";

export async function topUp({
  address,
  faucetEndpoint,
  rpcEndpoint,
  token = "NIL",
  amount = 1_000_000_000_000_000_000n,
}: {
  address: Hex;
  faucetEndpoint: string;
  rpcEndpoint: string;
  token?: string;
  amount?: bigint;
}): Promise<void> {
  const shardId = getShardIdFromAddress(address);

  const client = new PublicClient({
    transport: new HttpTransport({
      endpoint: rpcEndpoint,
    }),
    shardId: shardId,
  });

  const faucetClient = new FaucetClient({
    transport: new HttpTransport({
      endpoint: faucetEndpoint,
    }),
  });

  const faucets = await faucetClient.getAllFaucets();
  const faucet = faucets[token];

  await faucetClient.topUpAndWaitUntilCompletion(
    {
      faucetAddress: faucet,
      smartAccountAddress: address,
      amount: BigInt(amount),
    },
    client,
  );
}
