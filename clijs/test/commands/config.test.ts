import fs from "node:fs";
import { describe, expect } from "vitest";
import { ConfigKeys } from "../../src/common/config.js";
import { CliTest } from "../setup.js";

describe("config commands", () => {
  CliTest("tests config:set command", async ({ runCommand, cfgPath }) => {
    const testKey = "rpc_endpoint";
    const testValue = "test_value";

    const { stdout } = await runCommand(["config", "set", testKey, testValue]);
    expect(stdout).to.contains(`Set ${testKey} to ${testValue}`);

    const configContent = fs.readFileSync(cfgPath, "utf8");
    expect(configContent).to.contains(testKey);
    expect(configContent).to.contains(testValue);
  });

  CliTest("tests config:get command", async ({ runCommand, configManager }) => {
    const testKey = "rpc_endpoint";
    const testValue = "get_test_value";

    configManager.updateConfig(ConfigKeys.NilSection, testKey, testValue);

    const { stdout, result } = await runCommand(["config", "get", testKey]);
    expect(stdout).to.contains(testValue);
    expect(result).to.equal(testValue);
  });

  CliTest("tests config:get with non-existent key", async ({ runCommand }) => {
    const nonExistentKey = "rpc_endpoint_new";

    const { stdout, result } = await runCommand(["config", "get", nonExistentKey]);
    expect(stdout).to.contains(`Key ${nonExistentKey} not found`);
    expect(result).to.equal("");
  });

  CliTest("tests config:show command", async ({ runCommand, configManager, cfgPath }) => {
    const testKey = "rpc_endpoint";
    const testValue = "show_test_value";
    configManager.updateConfig(ConfigKeys.NilSection, testKey, testValue);

    const { result } = await runCommand(["config", "show"]);
    const configContent = fs.readFileSync(cfgPath, "utf8");

    expect(result).to.equal(configContent);
    expect(result).to.contains(testKey);
    expect(result).to.contains(testValue);
  });

  CliTest("tests config:init command", async ({ runCommand, cfgPath }) => {
    await runCommand(["config", "init"]);

    const updatedConfig = fs.readFileSync(cfgPath, "utf8");
    expect(updatedConfig).to.contains("rpc_endpoint");
  });
});
