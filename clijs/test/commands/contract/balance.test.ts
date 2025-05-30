import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("contract:balance", () => {
  CliTest("gets the balance of a contract", async ({ runCommand, smartAccount }) => {
    const contractAddress = smartAccount.address;

    const { result, stdout } = await runCommand(["contract", "balance", contractAddress]);

    expect(typeof result).toBe("string");
    expect(stdout).toContain("Balance: ");

    const { result: quietResult, stdout: quietStdout } = await runCommand([
      "contract",
      "balance",
      contractAddress,
      "-q",
    ]);

    expect(typeof quietResult).toBe("string");
    expect(quietStdout).not.toContain("Balance: ");
    expect(quietStdout.trim()).toBe((quietResult as bigint).toString());

    expect(result).toEqual(quietResult);
  });
});
