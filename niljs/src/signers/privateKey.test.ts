import { generateRandomKeyPair, generateRandomPrivateKey } from "./privateKey.js";

test("generatePrivateKey", async ({ expect }) => {
  const result = generateRandomPrivateKey();

  expect(result).toBeDefined();
});

test("generateKeyPair", async ({ expect }) => {
  const result = generateRandomKeyPair();

  expect(result).toBeDefined();
});
