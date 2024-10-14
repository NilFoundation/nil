import { HttpTransport, PublicClient, addHexPrefix } from "@nilfoundation/niljs";
import { config } from "../config";
import { bytesToHex } from "viem";

const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: config.RPC_URL,
  }),
  shardId: 1,
});

export const fetchAccountState = async (address: `0x${string}`) => {
  const refinedAddress = addHexPrefix(address);
  const [balance, currencies, code] = await Promise.all([
    client.getBalance(refinedAddress, "latest"),
    client.getCurrencies(refinedAddress, "latest"),
    client.getCode(refinedAddress, "latest").catch(() => {
      return Uint8Array.of();
    }),
  ]);

  return {
    balance: balance.toString(10),
    code: bytesToHex(code),
    isInitialized: code.length > 0,
    currencies,
  };
};
