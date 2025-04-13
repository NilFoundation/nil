import fs from "node:fs";
import * as ini from "ini";

export interface NilConfigIni {
  rpcEndpoint: string;
  cometaEndpoint: string;
  privateKey: string;
  address: string;
}

export function saveConfig(config: NilConfigIni, force: boolean): string {
  const path = process.env.NIL_CONFIG_INI ?? "~/.config/nil/config.ini";
  if (fs.existsSync(path) && !force) {
    throw new Error("config.ini already exists. Use --force to overwrite.");
  }

  const cfg = new Map<string, unknown>();
  cfg.set("address", config.address);
  cfg.set("rpc_endpoint", config.rpcEndpoint);
  cfg.set("cometa_endpoint", config.cometaEndpoint);
  cfg.set("private_key", config.privateKey);
  const configData = ini.stringify(Object.fromEntries(cfg));
  fs.writeFileSync(path, configData);
  return path;
}

export function fetchConfigIni(): NilConfigIni {
  const path = process.env.NIL_CONFIG_INI ?? "~/.config/nil/config.ini";
  if (!fs.existsSync(path)) {
    throw new Error("config.ini is not set. Please run `npx hardhat config-init`");
  }
  const configData = fs.readFileSync(path, "utf-8");
  const iniCfg = ini.parse(configData).nil;
  return {
    rpcEndpoint: iniCfg.rpc_endpoint,
    cometaEndpoint: iniCfg.cometa_endpoint,
    privateKey: iniCfg.private_key,
    address: iniCfg.address,
  };
}
