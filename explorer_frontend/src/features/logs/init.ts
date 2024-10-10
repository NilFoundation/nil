import { combine } from "effector";
import { compileCodeFx } from "../code/model";
import { $expandedLogs, $logs, LogTopic, LogType, collapseLog, expandLog } from "./model";
import { nanoid } from "nanoid";
import { callFx, deploySmartContractFx, sendMethodFx } from "../contracts/model";

$logs.on(callFx.failData, (logs, error) => {
  return [
    {
      id: nanoid(),
      topic: LogTopic.Call,
      type: LogType.Error,
      shortDescription: "Call failed",
      payload: {
        error: `${error}`,
      },
      timestamp: Date.now(),
    },
    ...logs,
  ];
});

$logs.on(deploySmartContractFx.doneData, (logs, { address }) => {
  return [
    {
      id: nanoid(),
      topic: LogTopic.Deployment,
      type: LogType.Success,
      shortDescription: "Contract deployed",
      payload: {
        address,
      },
      timestamp: Date.now(),
    },
    ...logs,
  ];
});

$logs.on(compileCodeFx.failData, (logs, error) => {
  return [
    {
      id: nanoid(),
      topic: LogTopic.Compilation,
      type: LogType.Error,
      shortDescription: "Compilation failed",
      payload: {
        error: `${error}`,
      },
      timestamp: Date.now(),
    },
    ...logs,
  ];
});

$logs.on(callFx.doneData, (logs, { result }) => {
  return [
    {
      id: nanoid(),
      topic: LogTopic.Call,
      type: LogType.Success,
      shortDescription: "Call successful",
      payload: {
        result,
      },
      timestamp: Date.now(),
    },
    ...logs,
  ];
});

$logs.on(callFx.failData, (logs, error) => {
  return [
    {
      id: nanoid(),
      topic: LogTopic.Call,
      type: LogType.Error,
      shortDescription: "Call failed",
      payload: {
        error: `${error}`,
      },
      timestamp: Date.now(),
    },
    ...logs,
  ];
});

$logs.on(sendMethodFx.doneData, (logs, { hash }) => {
  return [
    {
      id: nanoid(),
      topic: LogTopic.SendTx,
      type: LogType.Success,
      shortDescription: "Transaction sent",
      payload: {
        hash,
      },
      timestamp: Date.now(),
    },
    ...logs,
  ];
});

$logs.on(sendMethodFx.failData, (logs, error) => {
  return [
    {
      id: nanoid(),
      topic: LogTopic.SendTx,
      type: LogType.Error,
      shortDescription: "Transaction failed",
      payload: {
        error: `${error}`,
      },
      timestamp: Date.now(),
    },
    ...logs,
  ];
});

$logs.on(deploySmartContractFx.failData, (logs, error) => {
  return [
    {
      id: nanoid(),
      topic: LogTopic.Deployment,
      type: LogType.Error,
      shortDescription: "Deployment failed",
      payload: {
        error: `${error}`,
      },
      timestamp: Date.now(),
    },
    ...logs,
  ];
});

$expandedLogs.on(expandLog, (expandedLogs, id) => {
  return {
    ...expandedLogs,
    [id]: true,
  };
});

$expandedLogs.on(collapseLog, (expandedLogs, id) => {
  return {
    ...expandedLogs,
    [id]: false,
  };
});

const $logsReverse = $logs.map((logs) => logs.slice().reverse());

export const $logsWithOpen = combine($logsReverse, $expandedLogs, (logs, expandedLogs) => {
  return logs.map((log) => {
    return {
      ...log,
      isOpen: expandedLogs[log.id],
    };
  });
});
