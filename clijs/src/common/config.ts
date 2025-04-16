import * as fs from "node:fs";
import * as path from "node:path";
import * as fse from "fs-extra";
import * as ini from "ini";

export type Config = Record<string, string | Record<string, string>>;

class IniParser {
  private filePath: string;
  private fileStructure: string[] = [];
  private parsedData: Config = {}; // Stores lines as is (for comments and ordering)

  constructor(filePath: string) {
    this.filePath = filePath;
  }

  // Load the INI file and parse it
  public load(): Config {
    const content = fs.readFileSync(this.filePath, "utf8");
    const lines = content.split("\n");

    let currentSection = "";
    this.fileStructure = []; // Reset file structure

    for (const line of lines) {
      this.fileStructure.push(line); // Preserve the raw line

      const trimmed = line.trim();

      if (trimmed.startsWith(";") || trimmed.startsWith("#")) {
        // Comment line, preserve as-is
        continue;
      }

      if (trimmed.startsWith("[") && trimmed.endsWith("]")) {
        // New section
        currentSection = trimmed.slice(1, -1);
        if (!this.parsedData[currentSection]) {
          this.parsedData[currentSection] = {};
        }

        continue;
      }

      if (trimmed.includes("=")) {
        // Key-value pair
        let [key, value] = trimmed.split("=").map((part) => part.trim());
        value = value.split('"').join(""); // Remove quotes
        if (currentSection) {
          (this.parsedData[currentSection] as Record<string, string>)[key] = value;
        } else {
          // Handle global (unsectioned) keys
          this.parsedData[key] = value;
        }
      }
    }

    return this.parsedData;
  }

  // Save the INI file while preserving comments and structure
  public save(config: Config) {
    const newLines: string[] = [];

    let currentSection = "";

    for (const line of this.fileStructure) {
      const trimmed = line.trim();

      if (trimmed.startsWith(";") || trimmed.startsWith("#")) {
        // Preserve comments
        newLines.push(line);
        continue;
      }

      if (trimmed.startsWith("[") && trimmed.endsWith("]")) {
        // Section header
        currentSection = trimmed.slice(1, -1);
        newLines.push(line);
        continue;
      }

      if (trimmed.includes("=")) {
        const [key] = trimmed.split("=").map((part) => part.trim());
        if (
          currentSection &&
          (config[currentSection] as Record<string, string>)?.[key] !== undefined
        ) {
          newLines.push(`${key} = ${(config[currentSection] as Record<string, string>)[key]}`);
          delete (config[currentSection] as Record<string, string>)[key];
        } else if (!currentSection && config[key] !== undefined) {
          // Handle global (unsectioned) keys
          newLines.push(`${key} = ${config[key]}`);
          delete config[key];
        } else {
          // Preserve original line if key doesn't exist in the new config
          newLines.push(line);
        }
        continue;
      }

      // Preserve any other lines
      newLines.push(line);
    }

    // Write back to the file
    fs.writeFileSync(this.filePath, newLines.join("\n"), "utf-8");

    // Append newly added values
    fs.appendFileSync(this.filePath, ini.stringify(config, { whitespace: true }), "utf-8");
  }
}

const DefaultConfig = `; Configuration for interacting with the =nil; cluster
[nil]

; Specify the RPC endpoint of your cluster
; For example, if your cluster's RPC endpoint is at "http://127.0.0.1:8529", set it as below
; rpc_endpoint = "http://127.0.0.1:8529"

; Specify the RPC endpoint of your Cometa service
; Cometa service is not mandatory, you can leave it empty if you don't use it
; For example, if your Cometa's RPC endpoint is at "http://127.0.0.1:8529", set it as below
; cometa_endpoint = "http://127.0.0.1:8529"

; Specify the RPC endpoint of a Faucet service
; Faucet service is not mandatory, you can leave it empty if you don't use it
; For example, if your Faucet's RPC endpoint is at "http://127.0.0.1:8529", set it as below
; faucet_endpoint = "http://127.0.0.1:8529"

; Specify the private key used for signing external transactions to your smart account.
; You can generate a new key with "nil keygen new".
; private_key = "WRITE_YOUR_PRIVATE_KEY_HERE"

; Specify the address of your smart account to be the receiver of your external transactions.
; You can deploy a new smart account and save its address with "nil smart-account new".
; address = "0xWRITE_YOUR_ADDRESS_HERE"
`;

export enum ConfigKeys {
  NilSection = "nil",
  RpcEndpoint = "rpc_endpoint",
  CometaEndpoint = "cometa_endpoint",
  FaucetEndpoint = "faucet_endpoint",
  PrivateKey = "private_key",
  Address = "address",
}

class ConfigManager {
  private configFilePath: string;
  private parser: IniParser;

  constructor(configFilePath = "") {
    this.configFilePath = configFilePath;
    this.parser = new IniParser(this.configFilePath);

    // Ensure the directory exists
    fse.ensureDirSync(path.dirname(this.configFilePath));

    if (!fs.existsSync(this.configFilePath)) {
      fs.writeFileSync(this.configFilePath, DefaultConfig, "utf8");
    }
  }

  public getConfigValue(section: string, key: string, fallback?: string): string | undefined {
    const config = this.loadConfig();
    const sectionConfig = config[section] as Record<string, string>;
    return sectionConfig?.[key] ?? fallback;
  }

  public loadConfig(): Config {
    try {
      return this.parser.load();
    } catch (error) {
      console.error("Failed to load configuration:", error);
      return {};
    }
  }

  public saveConfig(config: Config) {
    try {
      this.parser.save(config);
    } catch (error) {
      console.error("Failed to save configuration:", error);
    }
  }

  public updateConfig(section: string, key: string, value: string) {
    const config = this.loadConfig();
    if (!config[section]) {
      config[section] = {};
    }

    (config[section] as Record<string, string>)[key] = value;
    this.saveConfig(config);
  }
}

export default ConfigManager;
