import { assertIsValidPrivateKey } from "../index.js";
import { privateKeyFromPhrase } from "./mnemonic.js";

test("privateKeyFromPhrase", async ({ expect }) => {
  const phrase = "test test test test test test test test test test test test";

  const privKey = privateKeyFromPhrase(phrase);

  assertIsValidPrivateKey(privKey);
  expect(privKey).toBeDefined();
});
