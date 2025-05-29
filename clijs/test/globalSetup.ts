import { type ChildProcess, exec, spawn } from "node:child_process";
import { closeSync, openSync } from "node:fs";
import { testEnv } from "./testEnv.js";

interface Nild {
  pid: number;
  process: ChildProcess;
  logFd: number;
}

let nildInstance: Nild;

export async function setup() {
  console.log("launching nild:", testEnv.nild);
  const logFd = openSync("nild.log", "w");
  await exec(COUNTER_COMPILATION_COMMAND);
  const nild = spawn(testEnv.nild, ["run", "--http-port", "8529", "--collator-tick-ms", "100"], {
    stdio: ["ignore", logFd, logFd],
  });

  await new Promise((resolve, reject) => {
    nild.once("error", reject);
    nild.once("spawn", resolve);
  });

  if (!nild.pid) {
    throw new Error("Failed to start nild");
  }

  await waitForServerReady(testEnv.endpoint, 30000); // 30 sec

  nildInstance = {
    pid: nild.pid,
    process: nild,
    logFd: logFd,
  };
}

export async function teardown() {
  console.log("stopping nild");

  nildInstance.process.kill("SIGTERM");
  await new Promise<void>((resolve) => {
    nildInstance.process.once("exit", () => {
      closeSync(nildInstance.logFd);
      resolve();
    });
  });
}

async function waitForServerReady(endpoint: string, timeout: number): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeout) {
    try {
      const response = await fetch(endpoint, { method: "GET" });
      if (response.ok) return;
    } catch (e) {}
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`Server not ready after ${timeout}ms`);
}

const COUNTER_COMPILATION_COMMAND =
  "solc -o ./test/contracts/Counter --bin --abi ./test/contracts/Counter.sol --overwrite";
