import { accounts } from "../../test/mocks/accounts.js";
import { addHexPrefix } from "../index.js";
import { getAddressFromPublicKey, getPublicKey } from "./publicKey.js";

test("getPublicKey", async ({ expect }) => {
  const account = accounts[0];
  const input = account.privateKey;
  const expectedOutput = addHexPrefix(account.publicKey);

  const result = getPublicKey(input, true);

  expect(result).toBe(expectedOutput);
});

test("getAddressFromPublicKey", async ({ expect }) => {
  const account = accounts[0];
  const input = account.publicKey;
  const expectedOutput = addHexPrefix(account.address);
  const accountShardId = account.shardId;

  const result = getAddressFromPublicKey(input, accountShardId);

  expect(result.toLowerCase()).toBe(expectedOutput.toLowerCase());
});
