import { describe, expect } from "vitest";
import ConfigManager from "../../../src/common/config.js";
import { ConfigKeys } from "../../../src/common/config.js";
import { CliTest } from "../../setup.js";

describe("smart-account:balance", () => {
  CliTest("runs smart-account:balance cmd", async ({ cfgPath, runCommand }) => {
    const {
      result,
      stdout: newStdout,
      stderr: newStderr,
    } = await runCommand(["smart-account", "new"]);
    expect(result).toBeTruthy();
    expect(newStdout).to.include("SmartAccount address");
    expect(newStderr).to.equal("");

    const configManager = new ConfigManager(cfgPath);
    expect(result).to.equal(
      configManager.getConfigValue(ConfigKeys.NilSection, ConfigKeys.Address),
    );

    const {
      result: balanceResult,
      stdout: balanceStdout,
      stderr: balanceStderr,
    } = await runCommand(["smart-account", "balance"]);
    expect(balanceResult).not.to.equal(0n);
    expect(balanceStdout).to.include("Balance");
    expect(balanceStderr).to.equal("");

    const invalidAddress = "0x1234567890123456789012345678901234567890";
    const { stdout: invalidStdout } = await runCommand([
      "smart-account",
      "balance",
      "--address",
      invalidAddress,
    ]);
    expect(invalidStdout).to.equal("");
  });
});
