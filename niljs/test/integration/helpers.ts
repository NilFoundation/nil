import { IndexerClient } from "../../src/clients/IndexerClient.js";
import {
  FaucetClient,
  HttpTransport,
  PublicClient,
  bytesToHex,
  calculateAddress,
  generateRandomPrivateKey,
  generateSmartAccount,
  getPublicKey,
  refineSalt,
  topUp,
} from "../../src/index.js";
import type { Hex } from "../../src/index.js";
import { testEnv } from "../testEnv.js";

export async function generateTestSmartAccount(shardId = 1) {
  return await generateSmartAccount({
    shardId: shardId,
    rpcEndpoint: testEnv.endpoint,
    faucetEndpoint: testEnv.faucetServiceEndpoint,
  });
}

export function generateRandomAddress(shardId = 1) {
  return bytesToHex(
    calculateAddress(
      shardId,
      new Uint8Array(1),
      refineSalt(BigInt(Math.floor(Math.random() * 10000))),
    ),
  );
}

export function generatePublicKey() {
  return getPublicKey(generateRandomPrivateKey());
}

export async function topUpTest(address: Hex, token = "NIL") {
  await topUp({
    address,
    rpcEndpoint: testEnv.endpoint,
    faucetEndpoint: testEnv.faucetServiceEndpoint,
    token,
  });
}

export function newClient(shardId = 1) {
  return new PublicClient({
    transport: new HttpTransport({
      endpoint: testEnv.endpoint,
    }),
    shardId,
  });
}

export function newFaucetClient() {
  return new FaucetClient({
    transport: new HttpTransport({
      endpoint: testEnv.faucetServiceEndpoint,
    }),
  });
}

export function newIndexerClient() {
  return new IndexerClient({
    transport: new HttpTransport({
      endpoint: testEnv.endpoint,
    }),
  });
}
