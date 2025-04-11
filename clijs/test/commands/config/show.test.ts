import { describe, expect } from "vitest";
import ConfigManager from "../../../src/common/config.js";
import { ConfigKeys } from "../../../src/common/config.js";
import { CliTest } from "../../setup.js";

describe("config:show", () => {
  CliTest("shows the contents of the config file", async ({ cfgPath, runCommand }) => {
    const configManager = new ConfigManager(cfgPath);
    const testKey1 = ConfigKeys.RpcEndpoint;
    const testValue1 = "http://test1.example.com:8529";
    const testKey2 = ConfigKeys.CometaEndpoint;
    const testValue2 = "http://test2.example.com:8529";

    configManager.updateConfig(ConfigKeys.NilSection, testKey1, testValue1);
    configManager.updateConfig(ConfigKeys.NilSection, testKey2, testValue2);

    const { result } = await runCommand(["config", "show"]);

    expect(result).toContain(`${testValue1}`);
    expect(result).toContain(`${testValue2}`);
  });
});
