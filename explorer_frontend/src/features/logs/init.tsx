import { compileCodeFx } from "../code/model";
import { $logs, LogTopic, LogType, clearLogs } from "./model";
import { nanoid } from "nanoid";
import { callFx, deploySmartContractFx, sendMethodFx } from "../contracts/model";
import { MonoParagraphMedium } from "baseui/typography";
import { formatSolidityError } from "./utils";
import { COLORS } from "@nilfoundation/ui-kit";
import {
  LabelMedium,
  StatefulTooltip,
  BUTTON_KIND,
  CopyButton,
  BUTTON_SIZE,
  ButtonIcon,
} from "@nilfoundation/ui-kit";
import { transactionRoute } from "../routing";
import { ContractDeployedLog } from "./components/ContractDeployedLog";
import { Link, ShareIcon } from "../shared";
import { LogTitleWithDetails } from "./components/LogTitleWithDetails";
import { TxDetials } from "./components/TxDetails";

$logs.on(deploySmartContractFx.doneData, (logs, { address, name, deployedFrom, txHash }) => {
  return [
    ...logs,
    {
      id: nanoid(),
      topic: LogTopic.Deployment,
      type: LogType.Success,
      shortDescription: (
        <LogTitleWithDetails
          title={
            <MonoParagraphMedium color={COLORS.green200}>
              {`Contract ${name} deployed from ${deployedFrom}`}
            </MonoParagraphMedium>
          }
          details={<TxDetials txHash={txHash} />}
        />
      ),
      payload: <ContractDeployedLog address={address} />,
      timestamp: Date.now(),
    },
  ];
});

$logs.on(deploySmartContractFx.failData, (logs, error) => {
  return [
    ...logs,
    {
      id: nanoid(),
      topic: LogTopic.Deployment,
      type: LogType.Error,
      shortDescription: (
        <MonoParagraphMedium color={COLORS.red200}>Deployment failed</MonoParagraphMedium>
      ),
      payload: <MonoParagraphMedium color={COLORS.red200}>{String(error)}</MonoParagraphMedium>,
      timestamp: Date.now(),
    },
  ];
});

$logs.on(compileCodeFx.doneData, (logs) => {
  return [
    ...logs,
    {
      id: nanoid(),
      topic: LogTopic.Compilation,
      type: LogType.Info,
      shortDescription: (
        <MonoParagraphMedium color={COLORS.gray400}>Compilation successful</MonoParagraphMedium>
      ),
      timestamp: Date.now(),
    },
  ];
});

$logs.on(compileCodeFx.failData, (logs, error) => {
  return [
    ...logs,
    {
      id: nanoid(),
      topic: LogTopic.Compilation,
      type: LogType.Error,
      shortDescription: (
        <MonoParagraphMedium color={COLORS.red200}>Compilation failed</MonoParagraphMedium>
      ),
      payload: (
        <MonoParagraphMedium color={COLORS.red200}>
          {formatSolidityError(String(error))}
        </MonoParagraphMedium>
      ),
      timestamp: Date.now(),
    },
  ];
});

$logs.on(callFx.doneData, (logs, { result, appName, functionName }) => {
  return [
    ...logs,
    {
      id: nanoid(),
      topic: LogTopic.Call,
      type: LogType.Success,
      shortDescription: (
        <MonoParagraphMedium
          color={COLORS.green200}
        >{`${appName}.${functionName}()`}</MonoParagraphMedium>
      ),
      payload: (
        <MonoParagraphMedium color={COLORS.gray400}>{`Result: ${result}`}</MonoParagraphMedium>
      ),
      timestamp: Date.now(),
    },
  ];
});

$logs.on(callFx.failData, (logs, error) => {
  return [
    ...logs,
    {
      id: nanoid(),
      topic: LogTopic.Call,
      type: LogType.Error,
      shortDescription: (
        <MonoParagraphMedium color={COLORS.red200}>Call failed</MonoParagraphMedium>
      ),
      payload: <MonoParagraphMedium color={COLORS.red200}>{String(error)}</MonoParagraphMedium>,
      timestamp: Date.now(),
    },
  ];
});

$logs.on(sendMethodFx.doneData, (logs, { hash, functionName, sendFrom, appName }) => {
  return [
    ...logs,
    {
      id: nanoid(),
      topic: LogTopic.SendTx,
      type: LogType.Success,
      shortDescription: (
        <LogTitleWithDetails
          title={
            <MonoParagraphMedium
              color={COLORS.green200}
            >{`${appName}.${functionName}() from ${sendFrom}`}</MonoParagraphMedium>
          }
          details={<TxDetials txHash={hash} />}
        />
      ),
      payload: (
        <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
          <LabelMedium color={COLORS.gray400}>Transaction hash:</LabelMedium>
          <LabelMedium color={COLORS.gray50}>{hash}</LabelMedium>
          <CopyButton kind={BUTTON_KIND.secondary} textToCopy={hash} size={BUTTON_SIZE.default} />
          <Link to={transactionRoute} params={{ hash }} target="_blank">
            <StatefulTooltip content="Open in Explorer" showArrow={false} placement="top">
              <ButtonIcon icon={<ShareIcon />} kind={BUTTON_KIND.secondary} />
            </StatefulTooltip>
          </Link>
        </div>
      ),
      timestamp: Date.now(),
    },
  ];
});

$logs.on(sendMethodFx.failData, (logs, error) => {
  return [
    ...logs,
    {
      id: nanoid(),
      topic: LogTopic.SendTx,
      type: LogType.Error,
      shortDescription: (
        <MonoParagraphMedium color={COLORS.red200}>Transaction failed</MonoParagraphMedium>
      ),
      payload: <MonoParagraphMedium color={COLORS.red200}>{String(error)}</MonoParagraphMedium>,
      timestamp: Date.now(),
    },
  ];
});

$logs.on(clearLogs, () => []);
