import { expect, vi } from "vitest";
import { PublicClient } from "@nilfoundation/niljs";
import { CliTest } from "../../setup.js";

describe("system:chain-id command", () => {
    CliTest("system chain-id command", async ({ runCommand }: { runCommand: (args: string[]) => Promise<any> }) => {
        const result = await runCommand(["system", "chain-id"]);
        expect(typeof result.result).toBe("number");
        expect(result.result).toBeGreaterThanOrEqual(0);
        expect(result.stderr).toBe("");
        expect(result.stdout).toBe("");
    });
});
