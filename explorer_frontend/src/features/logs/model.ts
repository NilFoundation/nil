import { createDomain } from "effector";
import type { ReactNode } from "react";

export const logsDomain = createDomain("logs");

export enum LogType {
  Warn = "warn",
  Error = "error",
  Info = "info",
  Success = "success",
}

export enum LogTopic {
  Compilation = "Compilation",
  Call = "Call",
  SendTx = "SendTx",
  Deployment = "Deployment",
}

export type Log = {
  id: string;
  topic: LogTopic;
  type: LogType;
  shortDescription: ReactNode;
  timestamp: number;
  payload?: ReactNode;
};

export const $logs = logsDomain.createStore<Log[]>([]);
