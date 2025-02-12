import { waitTillCompleted } from "../../src/index.js";
import { generateRandomAddress, newClient, newFaucetClient } from "./helpers.js";

const client = newClient();

test("Receipt test", async ({ expect }) => {
  const faucetClient = newFaucetClient();
  const faucets = await faucetClient.getAllFaucets();

  const smartAccountAddress = generateRandomAddress();

  const faucetHash = await faucetClient.topUp({
    faucetAddress: faucets.NIL,
    smartAccountAddress: smartAccountAddress,
    amount: 100n,
  });

  const receipts = await waitTillCompleted(client, faucetHash);

  expect(receipts).toBeDefined();
  for (const receipt of receipts) {
    expect(receipt).toBeDefined();
    expect(receipt.gasPrice).toBeDefined();
    expect(receipt.gasUsed).toBeDefined();
    expect(receipt.gasPrice).toBeTypeOf("bigint");
  }
});
