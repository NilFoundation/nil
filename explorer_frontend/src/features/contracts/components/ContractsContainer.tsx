import { Card } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { useSwipeable } from "react-swipeable";
import { getMobileStyles } from "../../../styleHelpers";
import { useMobile } from "../../shared";
import { $activeAppWithState, closeApp } from "../models/base";
import { Contracts } from "./Contracts/Contracts";
import { DeployContractModal } from "./Deploy/DeployContractModal";
import { ContractManagement } from "./Management/ContractManagement";

export const ContractsContainer = () => {
  const [css, theme] = useStyletron();
  const app = useUnit($activeAppWithState);
  const Component = app?.address ? ContractManagement : Contracts;
  const [isMobile] = useMobile();
  const handlers = useSwipeable({
    onSwipedLeft: () => closeApp(),
    onSwipedRight: () => closeApp(),
  });

  return (
    <Card
      {...handlers}
      overrides={{
        Root: {
          style: {
            maxWidth: isMobile ? "calc(100vw - 20px)" : "none",
            width: isMobile ? "100%" : "100%",
            height: "100%",
            backgroundColor: theme.colors.backgroundPrimary,
            paddingRight: "0",
            paddingLeft: "0",
            paddingBottom: "24px",
            overflow: "hidden",
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
  );
};
