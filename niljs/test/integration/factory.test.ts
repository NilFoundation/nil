import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { getContract, waitTillCompleted } from "../../src/index.js";
import { generateTestSmartAccount, newClient } from "./helpers.js";

const client = newClient();

beforeAll(async () => {
  const fileName = "./contracts/Incrementer.sol";
  const absolutePath = join(__dirname, fileName);
  const dirName = join(__dirname, "./contracts");
  await new Promise((resolve, reject) => {
    const p = spawn("solc", ["--overwrite", "--abi", "--bin", absolutePath, "-o", dirName], {});
    let stdout = "";
    let stderr = "";
    p.stdout.on("data", (data) => {
      stdout += data;
    });
    p.stderr.on("data", (data) => {
      stderr += data;
    });
    p.on("close", (code) => {
      if (code === 0) {
        resolve(stdout);
      } else {
        console.log("stdout", stdout);
        console.error("stderr", stderr);
        reject(stderr);
      }
    });
  });
});

test("Contract Factory", async ({ expect }) => {
  const bin = readFileSync(join(__dirname, "./contracts/Incrementer.bin"), "utf8");
  const abi = [
    {
      inputs: [{ internalType: "uint256", name: "start", type: "uint256" }],
      stateMutability: "nonpayable",
      type: "constructor",
    },
    {
      inputs: [
        { internalType: "uint256", name: "a", type: "uint256" },
        { internalType: "uint256", name: "b", type: "uint256" },
      ],
      name: "add",
      outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
      stateMutability: "pure",
      type: "function",
    },
    {
      inputs: [],
      name: "counter",
      outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
      stateMutability: "view",
      type: "function",
    },
    {
      inputs: [],
      name: "getCounter",
      outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
      stateMutability: "view",
      type: "function",
    },
    { inputs: [], name: "increment", outputs: [], stateMutability: "nonpayable", type: "function" },
    {
      inputs: [],
      name: "incrementExternal",
      outputs: [],
      stateMutability: "nonpayable",
      type: "function",
    },
    {
      inputs: [{ internalType: "uint256", name: "_counter", type: "uint256" }],
      name: "setCounter",
      outputs: [],
      stateMutability: "nonpayable",
      type: "function",
    },
    {
      inputs: [
        { internalType: "uint256", name: "hash", type: "uint256" },
        { internalType: "bytes", name: "signature", type: "bytes" },
      ],
      name: "verifyExternal",
      outputs: [{ internalType: "bool", name: "", type: "bool" }],
      stateMutability: "view",
      type: "function",
    },
    { stateMutability: "payable", type: "receive" },
  ] as const;

  const smartAccount = await generateTestSmartAccount();

  const { tx: deployTx, address: incrementerAddress } = await smartAccount.deployContract({
    abi: abi,
    bytecode: `0x${bin}`,
    args: [100n],
    salt: BigInt(Math.floor(Math.random() * 1000000)),
    shardId: 1,
  });

  await deployTx.wait();

  const incrementer = getContract({
    abi: abi,
    address: incrementerAddress,
    client,
    smartAccount,
    externalInterface: {
      signer: smartAccount.signer,
      methods: ["incrementExternal"],
    },
  } as const);

  const value = await incrementer.read.counter([]);

  expect(value).toBe(100n);
  const hash = await incrementer.write.increment([]);

  const receipts = await waitTillCompleted(client, hash);
  expect(receipts.some((receipt) => !receipt.success)).toBe(false);
  const newValue = await incrementer.read.counter([]);
  expect(newValue).toBe(101n);

  const tx11 = await smartAccount.sendTransaction({
    to: incrementerAddress,
    value: 100_000_000_000_000n,
  });
  const receipts11 = await tx11.wait();
  expect(receipts11.some((receipt) => !receipt.success)).toBe(false);
  const hash2 = await incrementer.external.incrementExternal([]);
  const receipts2 = await waitTillCompleted(client, hash2);
  expect(receipts2.some((receipt) => !receipt.success)).toBe(false);
  const newValue2 = await incrementer.read.counter([]);
  expect(newValue2).toBe(102n);
});
