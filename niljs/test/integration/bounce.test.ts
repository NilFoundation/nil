import { encodeFunctionData } from "viem";
import { SmartAccountV1 } from "../../src/index.js";
import { generateRandomAddress, generateTestSmartAccount, newClient } from "./helpers.js";

const client = newClient();

test("bounce", async () => {
  const gasPrice = await client.getGasPrice(1);

  const smartAccount = await generateTestSmartAccount();
  const anotherSmartAccount = await generateTestSmartAccount();

  const bounceAddress = generateRandomAddress();

  const tx = await smartAccount.sendTransaction({
    to: anotherSmartAccount.address,
    value: 10_000_000n,
    bounceTo: bounceAddress,
    feeCredit: 100_000n * gasPrice,
    data: encodeFunctionData({
      abi: SmartAccountV1.abi,
      functionName: "syncCall",
      args: [smartAccount.address, 100_000n, 10_000_000n, "0x"],
    }),
  });

  // const receipts = await waitTillCompleted(client, hash);
  const receipts = await tx.wait();

  expect(receipts.length).toBeDefined();
  expect(receipts.some((r) => !r.success)).toBe(true);

  expect(receipts.length).toBeGreaterThan(2);

  const balance = await client.getBalance(bounceAddress, "latest");

  expect(balance).toBeGreaterThan(0n);
});
