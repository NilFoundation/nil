import { nanoid } from "nanoid";
import { $smartAccount, sendTransactionFx } from "../features/account-connector/model";
import {
  triggerCustomConsoleLogEvent,
  triggerCustomConsoleWarnEvent,
} from "../features/code/model";

let workingWorker: Worker | null = null;

export const run = (code: string, workerHandler?: (event: MessageEvent) => void) => {
  const codeBlob = new Blob([code], { type: "text/javascript" });
  const codeUrl = URL.createObjectURL(codeBlob);

  if (workingWorker) {
    workingWorker.terminate();
  }

  const id = nanoid(6);

  workingWorker = new Worker(codeUrl, {
    type: "module",
    name: id,
  });

  if (workerHandler) {
    workingWorker.addEventListener("message", workerHandler);
  }

  workingWorker.addEventListener("message", (event) => {
    switch (event.data.type) {
      case "sendTransaction":
        console.log("Received sendTransaction message:", event.data);
        sendTransactionFx({ params: event.data.params, smartAccount: $smartAccount.getState()! })
          .then((transaction) => {
            const hash = transaction.hash;
            workingWorker.postMessage({
              type: "sendTransactionResult",
              result: hash,
              id: event.data.id,
              status: "ok",
            });
          })
          .catch((error) => {
            const message = error.message || "Unknown error";
            workingWorker.postMessage({
              type: "sendTransactionResult",
              result: message,
              id: event.data.id,
              status: "error",
            });
          });
        break;
      case "log":
        triggerCustomConsoleLogEvent(`Console log: ${event.data.args}`);
        break;
      case "warn":
        triggerCustomConsoleWarnEvent(`Console warning: ${event.data.args}`);
        break;
      case "error":
        triggerCustomConsoleWarnEvent(`Script error: ${event.data.args}`);
        break;
      default:
        console.warn("Unknown message type:", event.data);
        break;
    }
  });
};
