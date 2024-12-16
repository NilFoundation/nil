import { Command } from "@oclif/core";

export default class Keygen extends Command {
  static override description =
    "Generate a new key or generate a key from the provided hex private key";

  async run(): Promise<void> {
    await this.config.runCommand("help", ["keygen"]);
  }
}
