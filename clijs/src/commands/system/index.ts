import { Command } from "@oclif/core";

export default class SystemCommand extends Command {
    static override description = "Get system information";

    async run(): Promise<void> {
        await this.config.runCommand("help", ["system"]);
    }
}
