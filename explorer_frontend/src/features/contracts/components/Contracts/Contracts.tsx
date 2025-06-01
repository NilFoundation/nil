import { useUnit } from "effector-react";
import { $contractWithState, $contracts } from "../../models/base";
import "../../init";
import { COLORS, LabelMedium, Spinner } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { memo } from "react";
import { $smartAccount } from "../../../account-connector/model";
import { compileCodeFx } from "../../../code/model";
import { $rpcIsHealthy } from "../../../healthcheck/model";
import { Contract } from "./Contract";
import { SmartAccountNotConnectedWarning } from "./SmartAccountNotConnectedWarning";

const MemoizedWarning = memo(SmartAccountNotConnectedWarning);

export const Contracts = () => {
  const [deployedApps, contracts, compilingContracts, smartAccount, rpcIsHealthy] = useUnit([
    $contractWithState,
    $contracts,
    compileCodeFx.pending,
    $smartAccount,
    $rpcIsHealthy,
  ]);
  const [css] = useStyletron();
  const smartAccountExists = smartAccount !== null;

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        height: "100%",
        width: "100%",
      })}
    >
      <div
        className={css({
          height: "100%",
          width: "100%",
          overflowY: "auto",
        })}
      >
        {!smartAccountExists && <MemoizedWarning />}
        {contracts.length === 0 && (
          <div
            className={css({
              height: "100%",
              width: "100%",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              paddingLeft: "25%",
              paddingRight: "25%",
              textAlign: "center",
            })}
          >
            {compilingContracts ? (
              <Spinner />
            ) : (
              <LabelMedium color={COLORS.gray400}>
                Compile the code to handle smart contracts.
              </LabelMedium>
            )}
          </div>
        )}
        {contracts.map((contract, i) => {
          const appsToShow = smartAccountExists
            ? deployedApps.filter((app) => app.bytecode === contract.bytecode)
            : [];
          return (
            <Contract
              key={`${contract.bytecode}-${i}`}
              contract={contract}
              deployedApps={appsToShow}
              disabled={!smartAccountExists || !rpcIsHealthy}
            />
          );
        })}
      </div>
    </div>
  );
};
