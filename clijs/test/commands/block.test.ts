import type { Block } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../setup.js";

describe("block:get blocks", () => {
  CliTest("tests getting blocks", async ({ runCommand }) => {
    const block1 = (await runCommand(["block", "latest", "-s", "1"])).result as Block<boolean>;
    expect(block1).toBeTruthy();
  });
});
