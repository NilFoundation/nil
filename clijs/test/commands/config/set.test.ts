import { describe, expect } from "vitest";
import ConfigManager from "../../../src/common/config.js";
import { ConfigKeys } from "../../../src/common/config.js";
import { CliTest } from "../../setup.js";

describe("config:set", () => {
  CliTest("sets a value in the config file", async ({ cfgPath, runCommand }) => {
    const testKey = ConfigKeys.RpcEndpoint;
    const testValue = "http://test.example.com:8529";
    const { result } = await runCommand(["config", "set", testKey, testValue]);

    expect(result).toEqual(`Set "${testKey}" to "${testValue}"`);

    const configManager = new ConfigManager(cfgPath);
    const value = configManager.getConfigValue(ConfigKeys.NilSection, testKey);
    expect(value).toBe(testValue);
  });

  CliTest("returns undefined for unsupported key", async ({ runCommand }) => {
    const { result, stderr } = await runCommand(["config", "set", "unsupported_key", "value"]);

    expect(result).toBeUndefined();
  });
});
