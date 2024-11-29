import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import type { Abi } from "abitype";
import solc from "solc-typed-ast";
import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  convertEthToWei,
  generateRandomPrivateKey,
  getContract,
  waitTillCompleted,
} from "../../src/index.js";
import { testEnv } from "../testEnv.js";

const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: testEnv.endpoint,
  }),
  shardId: 1,
});

beforeAll(async () => {
  const fileName = "./contracts/Incrementer.sol";
  const absolutePath = join(__dirname, fileName);
  const res = await solc.compileSol(absolutePath, "auto");
  for (const fileName in res.data.contracts) {
    const item = res.data.contracts[fileName];
    for (const contractName in item) {
      const contract = item[contractName];
      writeFileSync(
        join(__dirname, `./contracts/${contractName}.bin`),
        contract.evm.bytecode.object,
      );
      writeFileSync(
        join(__dirname, `./contracts/${contractName}.abi.json`),
        JSON.stringify(contract.abi),
      );
    }
  }
});

test("Contract Factory", async ({ expect }) => {
  const bin = readFileSync(join(__dirname, "./contracts/Incrementer.bin"), "utf8");
  const abiJSON = readFileSync(join(__dirname, "./contracts/Incrementer.abi.json"), "utf8");
  const abi = JSON.parse(abiJSON) as Abi;

  const faucet = new Faucet(client);
  const key = generateRandomPrivateKey();
  const signer = new LocalECDSAKeySigner({
    privateKey: key,
  });

  const pubkey = signer.getPublicKey();

  const wallet = new WalletV1({
    pubkey: pubkey,
    salt: BigInt(Math.floor(Math.random() * 1000000)),
    shardId: 1,
    client,
    signer,
  });
  const walletAddress = wallet.address;

  await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(0.1));

  await wallet.selfDeploy(true);

  const { hash: deployHash, address: incrementerAddress } = await wallet.deployContract({
    abi: abi,
    bytecode: `0x${bin}`,
    args: [100n],
    salt: BigInt(Math.floor(Math.random() * 1000000)),
    shardId: 1,
  });

  await waitTillCompleted(client, deployHash);

  const incrementer = getContract({
    abi: abi as unknown[],
    address: incrementerAddress,
    client,
    wallet,
  });

  const value = await incrementer.read.counter([]);

  expect(value).toBe(100n);
  const hash = await incrementer.write.increment([]);

  const receipts = await waitTillCompleted(client, hash);
  expect(receipts.some((receipt) => !receipt.success)).toBe(false);
  const newValue = await incrementer.read.counter([]);
  expect(newValue).toBe(101n);
});
