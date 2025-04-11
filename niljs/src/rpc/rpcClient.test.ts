import { expect, test } from "vitest";
import { createRPCClient } from "./createRPCClient.js";

const endpoint = "http://127.0.0.1:8529";

test("creates a new RPC client with the correct endpoint", () => {
  const actualClient = createRPCClient(endpoint);

  expect(actualClient).not.toBeUndefined();
});

test("creates a new RPC client with the correct endpoint and headers", () => {
  const actualClient = createRPCClient(endpoint, {
    headers: {
      "My-header": "my-value",
    },
  });

  expect(actualClient).not.toBeUndefined();
});

test("throws an error when the headers are invalid (helps in JavaScript)", () => {
  const headers = {
    "Invalid-header": 333,
  } as unknown as Record<string, string>;

  expect(() => createRPCClient(endpoint, { headers })).toThrow();
});
