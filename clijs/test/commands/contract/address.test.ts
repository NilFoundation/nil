import * as fs from "node:fs";
import * as path from "node:path";
import { calculateAddress, hexToBytes, refineSalt } from "@nilfoundation/niljs";
import { bytesToHex } from "viem";
import { describe, expect } from "vitest";
import { bigIntReplacer } from "../../../src/base.js";
import { CliTest } from "../../setup.js";

describe("contract:address", () => {
  CliTest("calculates the contract address correctly", async ({ runCommand }) => {
    const abiPath = path.join(__dirname, "temp_test.abi");
    const binPath = path.join(__dirname, "temp_test.bin");

    const mockAbi = JSON.stringify(
      [
        {
          inputs: [],
          stateMutability: "nonpayable",
          type: "constructor",
        },
      ],
      bigIntReplacer,
    );

    const mockBin =
      "0x608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c80636d4ce63c1461003b578063e5c19b2d14610059575b600080fd5b610043610075565b60405161005091906100a1565b60405180910390f35b610073600480360381019061006e91906100ed565b61007e565b005b60008054905090565b8060008190555050565b6000819050919050565b61009b81610088565b82525050565b60006020820190506100b66000830184610092565b92915050565b600080fd5b6100ca81610088565b81146100d557600080fd5b50565b6000813590506100e7816100c1565b92915050565b600060208284031215610103576101026100bc565b5b6000610111848285016100d8565b9150509291505056fea2646970667358221220223a5935fa6b2d9b958082e0fd3a3a684994bf3f61152c4e5e6b8879d2f0f84e64736f6c634300080d0033";

    fs.writeFileSync(abiPath, mockAbi);
    fs.writeFileSync(binPath, mockBin);

    try {
      const { result, stdout } = await runCommand([
        "contract",
        "address",
        "--abiPath",
        abiPath,
        "--binPath",
        binPath,
        "--salt",
        "100",
        "--shardId",
        "1",
      ]);

      const constructorData = hexToBytes(mockBin);
      const expectedAddress = calculateAddress(1, constructorData, refineSalt(BigInt(100)));

      expect(result).toBe(bytesToHex(expectedAddress));
      expect(stdout).contains(bytesToHex(expectedAddress));
    } finally {
      if (fs.existsSync(abiPath)) fs.unlinkSync(abiPath);
      if (fs.existsSync(binPath)) fs.unlinkSync(binPath);
    }
  });
});
