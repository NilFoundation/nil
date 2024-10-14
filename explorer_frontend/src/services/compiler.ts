import { createEffect } from "effector";
import { type CompileWorker, solidityWorker } from "../features/shared/utils/solidityCompiler";

const poolWorker: Record<string, CompileWorker> = {};

export const fetchSolidityCompiler = createEffect(async (version: string) => {
  if (poolWorker[version]) {
    return poolWorker[version];
  }
  const worker = await solidityWorker({ version });
  poolWorker[version] = worker;
  return worker;
});
