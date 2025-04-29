import { type ChildProcess, spawn } from "node:child_process";
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
