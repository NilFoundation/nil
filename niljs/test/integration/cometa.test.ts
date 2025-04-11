import { CometaClient } from "../../src/clients/CometaClient.js";
import type { ContractData } from "../../src/clients/types/CometaTypes.js";
import { base64ToHex, toHex } from "../../src/encoding/toHex.js";
import type { SmartAccountV1 } from "../../src/smart-accounts/SmartAccountV1/SmartAccountV1.js";
import { HttpTransport } from "../../src/transport/HttpTransport.js";
import type { Hex } from "../../src/types/Hex.js";
import { convertEthToWei } from "../../src/utils/eth.js";
import { waitTillCompleted } from "../../src/utils/receipt.js";
import { testEnv } from "../testEnv.js";
import { generateTestSmartAccount } from "./helpers.js";

const incrementerContract = `
  // SPDX-License-Identifier: MIT
  pragma solidity ^0.8.28;
  contract Incrementer {
    uint256 public value;
    constructor() {
      value = 0;
    }
    function increment() public {
      value += 1;
    }
  }
`;

let contractAdress: Hex;
let contractCompilationResult: ContractData;
let sAcc: SmartAccountV1;

beforeAll(async () => {
  sAcc = await generateTestSmartAccount();
});

const cometaClient = new CometaClient({
  transport: new HttpTransport({
    endpoint: testEnv.cometaServiceEndpoint,
    timeout: 60 * 1000,
  }),
  shardId: 1,
});

test("compile contract", async () => {
  const res = await cometaClient.compileContract({
    contractName: "Incrementer:Incrementer",
    compilerVersion: "0.8.28",
    settings: {
      evmVersion: "cancun",
    },
    language: "Solidity",
    sources: {
      Incrementer: {
        content: incrementerContract,
      },
    },
  });
  expect(res).toBeDefined();
  expect(res.code).toBeDefined();
  expect(res.abi).toBeDefined();
  contractCompilationResult = res;
});

test("register contract", async () => {
  const {
    address,
    tx: { hash },
  } = await sAcc.deployContract({
    bytecode: base64ToHex(contractCompilationResult.initCode),
    abi: JSON.parse(contractCompilationResult.abi),
    args: [],
    feeCredit: convertEthToWei(0.00001),
    salt: BigInt(Math.floor(Math.random() * 10000000000000000)),
    shardId: 1,
  });

  const receipts = await waitTillCompleted(sAcc.client, hash);
  expect(receipts.some((receipt) => !receipt.success)).toBe(false);

  contractAdress = address;

  const code = await sAcc.client.getCode(address);

  expect(code).toBeDefined();

  const a = await cometaClient.registerContract(
    {
      contractName: "Incrementer:Incrementer",
      compilerVersion: "0.8.28",
      settings: {
        evmVersion: "cancun",
      },
      language: "Solidity",
      sources: {
        Incrementer: {
          content: incrementerContract,
        },
      },
    },
    address,
  );
});

test("get abi", async () => {
  const abi = await cometaClient.getAbi(contractAdress);

  expect(abi).toBeDefined();
  expect(abi).toBe(contractCompilationResult.abi);
});

test("get source code", async () => {
  const sourceCode = await cometaClient.getSourceCode(contractAdress);

  expect(sourceCode).toBeDefined();

  const { Incrementer } = sourceCode;

  expect(Incrementer).toBe(incrementerContract);
});

test("get contract", async () => {
  const contract = await cometaClient.getContract(contractAdress);

  expect(contract).toBeDefined();
  expect(contract.name).toBe("Incrementer:Incrementer");
  expect(contract.sourceCode.Incrementer).toBe(incrementerContract);
});

test("decode transactions calldata", async () => {
  const incrementFuncSelector = toHex(contractCompilationResult.methodIdentifiers["increment()"]);
  const txData = await cometaClient.decodeTransactionsCallData([
    {
      address: contractAdress,
      funcId: incrementFuncSelector,
    },
  ]);

  expect(txData).toBeDefined();
  expect(txData.includes("increment()")).toBe(true);
});
