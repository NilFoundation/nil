import type { Abi } from "abitype";
import { createCompileInput } from "./helper";
import { browserSolidityCompiler } from "./solidity.worker";

export type Task = {
  code: string;
  options?: {
    optimize?: boolean;
    runs?: number;
  };
};

export type CompiledContract = {
  abi: Abi;
  evm: {
    bytecode: {
      object: string;
    };
    methodIdentifiers: Record<string, string>;
  };
};

export type CompilationResult = {
  contracts?: {
    Compiled_Contracts: Record<string, CompiledContract>;
  };
  errors: {
    component: string;
    errorCode: number;
    formattedMessage: string;
    message: string;
    severity: string;
    sourceLocation: {
      end: number;
      file: string;
      start: number;
    };
    type: string;
  }[];
};

export class CompileWorker {
  worker: Worker;
  queue: Task[];
  currentTask: Task | null = null;
  promiseMap: Map<
    Task,
    {
      resolve: (result: CompilationResult) => void;
      reject: (error: Error) => void;
    }
  >;
  constructor(worker: Worker) {
    this.worker = worker;
    this.queue = [];
    this.promiseMap = new Map();
    this.worker.addEventListener("message", (event) => {
      const result = event.data as CompilationResult;

      if (
        // biome-ignore lint/suspicious/noPrototypeBuiltins: <explanation>
        !result.hasOwnProperty("contracts") ||
        // biome-ignore lint/suspicious/noPrototypeBuiltins: <explanation>
        (result.hasOwnProperty("errors") &&
          result.errors.filter((x) => x.severity !== "warning").length > 0)
      ) {
        const task = this.currentTask;
        if (task) {
          // biome-ignore lint/style/noNonNullAssertion: i check that task is not null
          const { reject } = this.promiseMap.get(task)!;
          const errorMsg = event.data.errors
            .map((error) => error.formattedMessage)
            .filter((message) => !message.startsWith("Warning"))
            .join("\n");
          reject(new Error(errorMsg));
          this.promiseMap.delete(task);
          this.currentTask = null;
          this._dequeue();
        }
      } else {
        const task = this.currentTask;
        if (task) {
          // biome-ignore lint/style/noNonNullAssertion: i check that task is not null
          const { resolve } = this.promiseMap.get(task)!;
          resolve(result);
          this.promiseMap.delete(task);
          this.currentTask = null;
          this._dequeue();
        }
      }
    });
  }

  async _dequeue() {
    if (this.currentTask) {
      return;
    }
    if (this.queue.length > 0) {
      const task = this.queue.shift();
      if (task) {
        console.log("task", task);
        this.currentTask = task;
        this.worker.postMessage({
          input: JSON.stringify(await createCompileInput(task.code, task.options)),
        });
      }
    }
  }

  compile(task: Task): Promise<CompilationResult> {
    return new Promise((resolve, reject) => {
      this.promiseMap.set(task, { resolve, reject });
      this.queue.push(task);
      this._dequeue();
    });
  }
}

export const solidityWorker = async ({ version }: { version: string }): Promise<CompileWorker> => {
  const worker = new Worker(
    URL.createObjectURL(new Blob([`(${browserSolidityCompiler})()`], { type: "module" })),
  );

  return new Promise((resolve, reject) => {
    worker.postMessage({ version });
    worker.onerror = reject;
    const installHandler = (event: MessageEvent) => {
      const { installVersion } = event.data;
      if (installVersion) {
        resolve(new CompileWorker(worker));
      }
      reject(new Error("Failed to install solidity compiler"));
      worker.removeEventListener("message", installHandler);
    };
    worker.addEventListener("message", installHandler);
  });
};

export const getCompilerVersions = async () => {
  return fetch("https://binaries.soliditylang.org/bin/list.json").then((response) =>
    response.json(),
  );
};
