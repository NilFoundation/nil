import { useUnit } from "effector-react";
import { $contractWithState, $contracts } from "../../models/base";
import "../../init";
import {
  BUTTON_KIND,
  BUTTON_SIZE,
  SPACE,
  Button,
  COLORS,
  LabelMedium,
  Spinner,
  ArrowUpIcon,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import { compileCodeFx } from "../../../code/model";
import { useMobile } from "../../../shared";
import { LayoutComponent, setActiveComponent } from "../../../../pages/sandbox/model";
import { Contract } from "./Contract";

export const Contracts = () => {
  const [deployedApps, contracts, compilingContracts] = useUnit([
    $contractWithState,
    $contracts,
    compileCodeFx.pending,
  ]);
  const [css] = useStyletron();
  const [isMobile] = useMobile();

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
                },
              },
            }}
            kind={BUTTON_KIND.secondary}
            size={BUTTON_SIZE.compact}
            onClick={() => setActiveComponent(LayoutComponent.Code)}
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
          return (
            <Contract
              key={`${contract.bytecode}-${i}`}
              contract={contract}
              deployedApps={deployedApps.filter((app) => app.bytecode === contract.bytecode)}
            />
          );
        })}
      </div>
    </div>
  );
};
