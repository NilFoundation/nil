import { createEvent, createStore } from "effector";

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
  shortDescription: string;
  timestamp: number;
  // biome-ignore lint/suspicious/noExplicitAny: i dunno
  payload: Record<string, any>;
};

export const $logs = createStore<Log[]>([]);
export const expandLog = createEvent<string>();
export const collapseLog = createEvent<string>();
export const $expandedLogs = createStore<Record<string, boolean>>({});
