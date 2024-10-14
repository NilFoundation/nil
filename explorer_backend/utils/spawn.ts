import { spawn } from "node:child_process";

export const call = async (cmd: string, args: string[]): Promise<string> => {
  console.log("call", `${cmd} ${args.join(" ")}`);
  return new Promise((resolve, reject) => {
    const p = spawn(cmd, args);
    let stdout = "";
    let stderr = "";
    p.stdout.on("data", (data) => {
      stdout += data;
    });
    p.stderr.on("data", (data) => {
      stderr += data;
    });
    p.on("close", (code) => {
      if (code === 0) {
        resolve(stdout);
      } else {
        console.log("stdout", stdout);
        console.error("stderr", stderr);
        reject(stderr);
      }
    });
  });
};
