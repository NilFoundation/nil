import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("contract:deploy", () => {
  CliTest("deploys a smart contract", async ({ runCommand }) => {
    const abiPath = "./test/contracts/Counter/Counter.abi";
    const binPath = "./test/contracts/Counter/Counter.bin";

    const result = await runCommand([
      "contract",
      "deploy",
      binPath,
      "constructor",
      "--abiPath",
      abiPath,
      "--salt",
      "42",
      "--feeCredit",
      "1000000000000",
    ]);

    expect(typeof result.result).toBe("string");
    expect((result.result as string).startsWith("0x")).toBe(true);
    expect((result.result as string).length).toBe(42);
    expect(result.stdout).contains("Transaction hash");
  });
});
