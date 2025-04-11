import { nanoid } from "nanoid";
import { triggerCustomConsoleLogEvent, triggerCustomConsoleWarnEvent } from "../features/code/model";

const interceptor = `
  (function() {
    const originalLog = console.log;
    const originalWarn = console.warn;
    console.log = function(...args) {
      self.postMessage({ type: 'log', args });
      originalLog.apply(console, args);
    };
    console.warn = function(...args) {
      self.postMessage({ type: 'warn', args });
      originalWarn.apply(console, args);
    };
  })();
`;

let workingWorker: Worker | null = null;

export const run = (code: string) => {
  const codeWithInterceptor = `${interceptor}\n${code}`;

  const codeBlob = new Blob([codeWithInterceptor], { type: "text/javascript" });
  const codeUrl = URL.createObjectURL(codeBlob);

  if (workingWorker) {
    workingWorker.terminate();
  }

  const id = nanoid(6);

  workingWorker = new Worker(codeUrl, {
    type: "module",
    name: id,
  });

  workingWorker.addEventListener("message", (event) => {
    const { type, args } = event.data;
    const parsedArgs = JSON.stringify(args);
    if (type === 'log') {
      triggerCustomConsoleLogEvent(`Console log: ${parsedArgs.toString()}`);
    } else {
      triggerCustomConsoleWarnEvent(`Console warning: ${parsedArgs.toString()}`);
    }
  });
}