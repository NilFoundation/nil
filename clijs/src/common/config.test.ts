import * as fs from "node:fs";
import { describe, expect } from "vitest";
import { CliTest } from "../../test/setup.js";
import ConfigManager, { ConfigKeys } from "./config.js";

describe("ConfigManager", () => {
  CliTest("should create a default config file if it does not exist", async ({ cfgPath }) => {
    const configManager = new ConfigManager(cfgPath);
    expect(fs.existsSync(cfgPath)).toBe(true);
  });

  CliTest("should load the default config", async ({ cfgPath }) => {
    const configManager = new ConfigManager(cfgPath);
    const config = configManager.loadConfig();
    expect(config).toHaveProperty("nil");
  });

  CliTest("should get a config value", async ({ cfgPath }) => {
    const configManager = new ConfigManager(cfgPath);
    configManager.updateConfig("nil", ConfigKeys.RpcEndpoint, "http://127.0.0.1:8529");
    const value = configManager.getConfigValue("nil", ConfigKeys.RpcEndpoint);
    expect(value).toBe("http://127.0.0.1:8529");

    const fallbackValue = configManager.getConfigValue("nil", ConfigKeys.CometaEndpoint, value);
    expect(fallbackValue).toBe(value);
  });

  CliTest("should update a config value", async ({ cfgPath }) => {
    const configManager = new ConfigManager(cfgPath);
    configManager.updateConfig("nil", ConfigKeys.RpcEndpoint, "http://127.0.0.1:1010");
    const config = configManager.loadConfig();
    expect(config.nil).toHaveProperty(ConfigKeys.RpcEndpoint, "http://127.0.0.1:1010");
  });

  CliTest("should preserve comments and structure when saving", async ({ cfgPath }) => {
    const initialContent = `; Comment line
[nil]
rpc_endpoint = http://127.0.0.1:8529
`;
    fs.writeFileSync(cfgPath, initialContent, "utf8");

    const configManager = new ConfigManager(cfgPath);
    configManager.updateConfig("nil", ConfigKeys.CometaEndpoint, "http://127.0.0.1:1234");
    const content = fs.readFileSync(cfgPath, "utf8");

    expect(content).toContain("; Comment line");
    expect(content).toContain("rpc_endpoint = http://127.0.0.1:8529");
    expect(content).toContain("cometa_endpoint = http://127.0.0.1:1234");
  });
});
