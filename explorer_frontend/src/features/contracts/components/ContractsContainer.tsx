import {
  ArrowUpIcon,
  BUTTON_KIND,
  BUTTON_SIZE,
  Button,
  COLORS,
  Card,
  LabelMedium,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { useSwipeable } from "react-swipeable";
import { getMobileStyles } from "../../../styleHelpers";
import { AccountPane } from "../../account-connector/components/AccountPane";
import { clickOnBackButton } from "../../code/model";
import { useMobile } from "../../shared";
import { $activeAppWithState, closeApp } from "../models/base";
import { Contracts } from "./Contracts/Contracts";
import { DeployContractModal } from "./Deploy/DeployContractModal";
import { ContractManagement } from "./Management/ContractManagement";

export const ContractsContainer = () => {
  const app = useUnit($activeAppWithState);
  const Component = app?.address ? ContractManagement : Contracts;
  const [isMobile] = useMobile();
  const [css, theme] = useStyletron();
  const handlers = useSwipeable({
    onSwipedLeft: () => closeApp(),
    onSwipedRight: () => closeApp(),
  });

  return (
    <div
      className={css({
        height: "100%",
        width: "100%",
      })}
    >
      {isMobile && (
        <div
          className={css({
            display: "flex",
            gap: "12px",
            marginBottom: "12px",
            alignItems: "center",
            width: "100%",
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
          <AccountPane />
        </div>
      )}
      <Card
        {...handlers}
        overrides={{
          Root: {
            style: {
              maxWidth: isMobile ? "calc(100vw - 20px)" : "none",
              width: "100%",
              height: "100%",
              backgroundColor: theme.colors.backgroundPrimary,
              paddingRight: "0",
              paddingLeft: "0",
              paddingBottom: "24px",
              overflow: "hidden",
              ...expandProperty("borderRadius", "16px"),
              ...getMobileStyles({
                paddingTop: "8px",
              }),
            },
          },
          Contents: {
            style: {
              height: "100%",
              maxWidth: "100%",
              width: "100%",
              paddingRight: "24px",
              paddingLeft: "24px",
              overscrollBehavior: "contain",
              ...getMobileStyles({
                height: "calc(100vh - 154px)",
                paddingLeft: "16px",
                paddingRight: "16px",
              }),
            },
          },
          Body: {
            style: {
              height: "100%",
              width: "100%",
              maxWidth: "100%",
            },
          },
        }}
      >
        <DeployContractModal
          isOpen={!!app && !app?.address}
          onClose={() => closeApp()}
          name={app?.name ?? "Deploy settings"}
        />
        <Component />
      </Card>
    </div>
  );
};
