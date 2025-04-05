import { Flags } from "@oclif/core";
import { BaseCommand } from "../../base.js";
import { logger } from "../../logger.js";
import * as fs from "node:fs";
import * as path from "node:path";

// Default config template
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

export default class ConfigInit extends BaseCommand {
    static override description = "Initialize the config file";

    static override examples = ["$ nil config init "];

    static override flags = {
        force: Flags.boolean({
            char: "f",
            description: "Force initialization even if the config file already exists",
            required: false,
            default: false,
        }),
    };

    public async run(): Promise<string> {
        const { flags } = await this.parse(ConfigInit);
        const { force } = flags;

        if (!this.configManager) {
            throw new Error("Config manager is not initialized");
        }

        const configPath = this.configManager["configFilePath"];

        if (fs.existsSync(configPath) && !force) {
            throw new Error(`Config file already exists at ${configPath}. Use --force to overwrite.`);
        }

        // Ensure the directory exists
        const dirPath = path.dirname(configPath);
        if (!fs.existsSync(dirPath)) {
            fs.mkdirSync(dirPath, { recursive: true });
        }

        // Write the default config
        fs.writeFileSync(configPath, DefaultConfig, "utf8");

        if (this.quiet) {
            this.log(configPath);
        } else {
            this.log(`The config file has been initialized successfully: ${configPath}`);
        }

        return configPath;
    }
}
