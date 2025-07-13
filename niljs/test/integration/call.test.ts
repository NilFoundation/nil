import type { Abi } from "abitype";
import { expect } from "vitest";
import { generateTestSmartAccount, newClient } from "./helpers.js";

const client = newClient();

const abi: Abi = [
  {
    inputs: [{ internalType: "int32", name: "val", type: "int32" }],
    name: "add",
    outputs: [],
    stateMutability: "nonpayable",
    type: "function",
  },
  {
    inputs: [],
    name: "get",
    outputs: [{ internalType: "int32", name: "", type: "int32" }],
    stateMutability: "view",
    type: "function",
  },
  {
    inputs: [
      { internalType: "uint256", name: "", type: "uint256" },
      { internalType: "bytes", name: "", type: "bytes" },
    ],
    name: "verifyExternal",
    outputs: [{ internalType: "bool", name: "", type: "bool" }],
    stateMutability: "pure",
    type: "function",
  },
];

test("Call counter status", async () => {
  const smartAccount = await generateTestSmartAccount();

  const { address, tx } = await smartAccount.deployContract({
    shardId: 1,
    bytecode:
      "0x608080604052346015576101ac908161001a8239f35b5f80fdfe6080806040526004361015610012575f80fd5b5f3560e01c9081633fa4f245146101905750806357b8a50f1461012c5780636d4ce63c146100e4578063796d7f561461008c576380c22d8a14610053575f80fd5b34610088576020366003190112610088576004358060030b81036100885763ffffffff195f54169063ffffffff16175f555f80f35b5f80fd5b346100885760403660031901126100885760243567ffffffffffffffff8111610088573660238201121561008857806004013567ffffffffffffffff8111610088573691016024011161008857602060405160018152f35b34610088575f3660031901126100885760205f5460030b7fff6fced5db6a6a3facba16ea5d832f4bda4c79c3844544f58b8587a941b2c9e182604051838152a1604051908152f35b34610088576020366003190112610088576004358060030b809103610088575f54908160030b01637fffffff198112637fffffff82131761017c5763ffffffff169063ffffffff1916175f555f80f35b634e487b7160e01b5f52601160045260245ffd5b34610088575f366003190112610088576020905f5460030b8152f3",
    salt: BigInt(Math.floor(Math.random() * 1000000000)),
    abi,
    args: [],
  });

  const receipt = await tx.wait({ waitTillMainShard: true });
  expect(receipt.length).toBeDefined();
  expect(receipt[0].success).toBe(true);

  const res = await client.call(
    {
      to: address,
      abi,
      functionName: "get",
    },
    "latest",
  );

  expect(res.decodedData).toEqual(0);

  const transactionHash = await smartAccount.sendTransaction({
    to: address,
    abi,
    functionName: "add",
    args: [100],
    value: 0n,
  });
  await transactionHash.wait();

  const resAfter = await client.call(
    {
      to: address,
      abi,
      functionName: "get",
    },
    "latest",
  );

  expect(resAfter.decodedData).toEqual(100);

  const syncTransactionHash = await smartAccount.syncSendTransaction({
    to: address,
    abi,
    functionName: "add",
    args: [100],
    value: 0n,
    gas: 1000000n,
    maxPriorityFeePerGas: 10n,
    maxFeePerGas: 1_000_000_000_000n,
  });

  const receipts = await syncTransactionHash.wait({ waitTillMainShard: true });

  const resAfterSync = await client.call(
    {
      to: address,
      abi,
      functionName: "get",
    },
    "latest",
  );

  expect(resAfterSync.decodedData).toEqual(200);
});
