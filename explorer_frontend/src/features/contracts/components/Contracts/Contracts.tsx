import { useUnit } from "effector-react";
import { $contractWithState, $contracts } from "../../models/base";
import "../../init";
import {
  ArrowUpIcon,
  BUTTON_KIND,
  BUTTON_SIZE,
  Button,
  COLORS,
  LabelMedium,
  SPACE,
  Spinner,
} from "@nilfoundation/ui-kit";
import { memo } from "react";
import { useStyletron } from "styletron-react";
import { $smartAccount } from "../../../account-connector/model";
import { clickOnBackButton, compileCodeFx } from "../../../code/model";
import { $rpcIsHealthy } from "../../../healthcheck/model";
import { useMobile } from "../../../shared";
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
  const [isMobile] = useMobile();
  const smartAccountExists = smartAccount !== null;

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        height: "100%",
      })}
    >
      {isMobile && (
        <div
          className={css({
            display: "flex",
            gap: "12px",
            marginBottom: SPACE[12],
            alignItems: "center",
          })}
        >
          <Button
            className={css({
              width: "32px",
              height: "32px",
            })}
            overrides={{
              Root: {
                style: {
                  paddingLeft: 0,
                  paddingRight: 0,
                  backgroundColor: theme.colors.backgroundSecondary,
                  ":hover": {
                    backgroundColor: theme.colors.backgroundTertiary,
                  },
                },
              },
            }}
            kind={BUTTON_KIND.secondary}
            size={BUTTON_SIZE.compact}
            onClick={() => clickOnBackButton()}
          >
            <ArrowUpIcon
              size={12}
              className={css({
                transform: "rotate(-90deg)",
              })}
            />
          </Button>
          <LabelMedium color={COLORS.gray50}>Contracts</LabelMedium>
        </div>
      )}
      <div
        className={css({
          height: "100%",
        })}
      >
        {!smartAccountExists && <MemoizedWarning />}
        {contracts.length === 0 && (
          <div
            className={css({
              height: "100%",
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
