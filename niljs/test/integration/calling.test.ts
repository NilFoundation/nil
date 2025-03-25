import { generateRandomAddress, generateTestSmartAccount, newClient } from "./helpers.js";

const client = newClient();

test("Async call to another shard send value", async () => {
  const smartAccount = await generateTestSmartAccount(1);

  const anotherAddress = generateRandomAddress(2);

  const gasPriceOnShard2 = await client.getGasPrice(2);

  const tx = await smartAccount.sendTransaction({
    to: anotherAddress,
    value: 50_000_000n,
    feeCredit: 100_000n * gasPriceOnShard2,
  });

  const receipts = await tx.wait();

  expect(receipts).toBeDefined();
  expect(receipts.some((r) => !r.success)).toBe(false);

  const balance = await client.getBalance(anotherAddress, "latest");
  expect(balance).toBeGreaterThan(0n);
});

test("sync call same shard send value", async () => {
  const smartAccount = await generateTestSmartAccount(1);

  const anotherAddress = generateRandomAddress(1);

  const tx = await smartAccount.syncSendTransaction({
    to: anotherAddress,
    value: 10n,
    gas: 100000n,
    maxPriorityFeePerGas: 10n,
    maxFeePerGas: 1_000_000_000_000n,
  });

  const receipts = await tx.wait();

  expect(receipts).toBeDefined();
  expect(receipts.some((r) => !r.success)).toBe(false);
  const balance = await client.getBalance(anotherAddress, "latest");

  expect(balance).toBe(10n);
});
