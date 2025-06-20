import { HttpTransport, addHexPrefix } from "@nilfoundation/niljs";
import { PublicClient } from "@nilfoundation/niljs";
import { config } from "../config";
import { bytesToHex } from "viem";
import { getShardIdsList } from "../daos/shards";

const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: config.RPC_URL,
    timeout: 10000,
  }),
  shardId: 1,
});

export const fetchAccountState = async (address: `0x${string}`) => {
  const refinedAddress = addHexPrefix(address);
  const [balance, tokens, code] = await Promise.all([
    client.getBalance(refinedAddress, "latest"),
    client.getTokens(refinedAddress, "latest"),
    client.getCode(refinedAddress, "latest").catch(() => {
      return Uint8Array.of();
    }),
  ]);

  return {
    balance: balance.toString(10),
    code: bytesToHex(code),
    isInitialized: code.length > 0,
    tokens,
  };
};

export const fetchShardsGasPrice = async () => {
  const shardsIds = await getShardIdsList();
  const promises = shardsIds.map((shardId) => client.getGasPrice(shardId));

  const gasPrices = await Promise.all(promises);
  const gasPriceMap = Object.fromEntries(
    shardsIds.map((shardId, index) => [shardId, gasPrices[index].toString(10)]),
  );

  return gasPriceMap;
};
