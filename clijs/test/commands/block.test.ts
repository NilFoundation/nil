import type { Block } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../setup.js";

// To run this test you need to run the nild:
// nild run --http-port 8529
// TODO: Setup nild automatically before running the tests
describe("block:get blocks", () => {
  CliTest("tests getting blocks", async ({ runCommand }) => {
    const block1 = (await runCommand(["block", "latest", "-s", "1"])).result as Block<boolean>;
    expect(block1).toBeTruthy();

    const block2 = (await runCommand(["block", "1", "-s", "1"])).result as Block<boolean>;
    expect(block2).toBeTruthy();
  });
});
