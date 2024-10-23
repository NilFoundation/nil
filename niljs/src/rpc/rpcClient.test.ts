import { expect, test } from "vitest";
import { createRPCClient } from "./rpcClient.js";

const endpoint = "http://127.0.0.1:8529";

test("creates a new RPC client with the correct endpoint", () => {
  const actualClient = createRPCClient(endpoint);

  expect(actualClient).not.toBeUndefined();
});
