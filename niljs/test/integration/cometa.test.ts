import type { ContractData } from "../../src/clients/types/CometaTypes.js";
import { base64ToHex } from "../../src/encoding/toHex.js";
import {
  CometaService,
  type Hex,
  HttpTransport,
  convertEthToWei,
  waitTillCompleted,
} from "../../src/index.js";
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

const sAcc = await generateTestSmartAccount();

const cometaService = new CometaService({
  transport: new HttpTransport({
    endpoint: testEnv.cometaServiceEndpoint,
  }),
  shardId: 1,
});

test("compile contract", async () => {
  const res = await cometaService.compileContract({
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
  const { address, hash } = await sAcc.deployContract({
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
  expect(async () => {
    await cometaService.registerContract(
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
  }).not.toThrow();
});

test("get abi", async () => {
  const abi = await cometaService.getAbi(contractAdress);
  expect(abi).toBeDefined();
  expect(abi).toBe(contractCompilationResult.abi);
});

test("get source code", async () => {
  const sourceCode = await cometaService.getSourceCode(contractAdress);
  expect(sourceCode).toBeDefined();
  const { Incrementer } = sourceCode;
  expect(Incrementer).toBe(incrementerContract);
});

test("get contract", async () => {
  const contract = await cometaService.getContract(contractAdress);
  expect(contract).toBeDefined();
  expect(contract.name).toBe("Incrementer:Incrementer");
  expect(contract.sourceCode.Incrementer).toBe(incrementerContract);
});
