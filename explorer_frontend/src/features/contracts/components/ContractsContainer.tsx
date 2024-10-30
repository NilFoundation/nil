import { useUnit } from "effector-react";
import { Contracts } from "./Contracts/Contracts";
import { ContractManagement } from "./Management/ContractManagement";
import { $activeAppWithState, closeApp } from "../model";
import { useSwipeable } from "react-swipeable";
import { COLORS, Card } from "@nilfoundation/ui-kit";
import { DeployContractModal } from "./Deploy/DeployContractModal";
import { getMobileStyles } from "../../../styleHelpers";

export const ContractsContainer = () => {
  const app = useUnit($activeAppWithState);
  const Component = app?.address ? ContractManagement : Contracts;
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
            maxWidth: "none",
            height: "100%",
            backgroundColor: COLORS.gray900,
            paddingRight: "0",
            paddingLeft: "0",
            paddingBottom: "24px",
          },
        },
        Contents: {
          style: {
            height: "100%",
            maxWidth: "none",
            width: "100%",
            paddingRight: "24px",
            paddingLeft: "24px",
            overflow: "auto",
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
            maxWidth: "none",
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
