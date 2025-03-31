import fs from "node:fs";
import "dotenv";
import * as ini from "ini";

let config: Map<string, unknown> | null = null;

export function getValue(key: string): string | undefined {
  if (config == null) {
    config = initConfig();
  }
  const v = config.get(key);
  if (v == null) {
    return undefined;
  }
  return v as string;
}

export function setValue(key: string, value: unknown): void {
  if (!process.env.NIL_CONFIG_INI) {
    throw new Error("NIL_CONFIG_INI is not set");
  }
  if (config == null) {
    config = initConfig();
  }
  config.set(key, value);
  const cfg = new Map<string, unknown>();
  cfg.set("nil", Object.fromEntries(config));
  const configData = ini.stringify(Object.fromEntries(cfg));
  fs.writeFileSync(process.env.NIL_CONFIG_INI, configData);
}

function initConfig() {
  const cfg = new Map<string, unknown>();

  if (process.env.WALLET_ADDR) {
    cfg.set("address", process.env.WALLET_ADDR);
  }
  if (process.env.PRIVATE_KEY) {
    cfg.set("private_key", process.env.PRIVATE_KEY);
  }
  if (process.env.NIL_RPC_ENDPOINT) {
    cfg.set("rpc_endpoint", process.env.NIL_RPC_ENDPOINT);
  }
  if (process.env.COMETA_ENDPOINT) {
    cfg.set("cometa_endpoint", process.env.COMETA_ENDPOINT);
  }

  // If the config file exists, read it and override the environment variables.
  if (process.env.NIL_CONFIG_INI && fs.existsSync(process.env.NIL_CONFIG_INI)) {
    const configData = fs.readFileSync(process.env.NIL_CONFIG_INI, "utf-8");
    const iniCfg = ini.parse(configData).nil;
    for (const key in iniCfg) {
      cfg.set(key, iniCfg[key]);
    }
  }
  return cfg;
}
