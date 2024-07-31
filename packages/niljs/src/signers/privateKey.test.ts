import { generateRandomPrivateKey } from "./privateKey.js";

test("generatePrivateKey", async ({ expect }) => {
  const result = generateRandomPrivateKey();

  expect(result).toBeDefined();
});
