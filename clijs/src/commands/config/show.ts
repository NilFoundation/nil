import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";
import * as fs from "node:fs";

export default class ConfigShow extends BaseCommand {
    static override description = "Show the contents of the config file";

    static override examples = ["$ nil config show"];

    public async run(): Promise<string> {
        if (!this.configManager) {
            throw new Error("Config manager is not initialized");
        }

        const configPath = this.configManager["configFilePath"];

        if (!fs.existsSync(configPath)) {
            throw new Error(`Config file not found at ${configPath}`);
        }

        const config = this.configManager.loadConfig();
        const nilSection = config[ConfigKeys.NilSection] as Record<string, string>;

        // Print the config file path
        this.log(`The config file: ${configPath}\n`);

        // Build the formatted output
        let formattedOutput = ``;

        if (nilSection) {
            for (const [key, value] of Object.entries(nilSection)) {
                formattedOutput += `${key.padEnd(18)}: ${value}\n`;
            }
        }

        return formattedOutput;
    }
}
