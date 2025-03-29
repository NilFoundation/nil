import { BUTTON_KIND, BUTTON_SIZE, COLORS, Tab, Tabs } from "@nilfoundation/ui-kit";
import { Button } from "baseui/button";
import { useUnit } from "effector-react";
import { useSwipeable } from "react-swipeable";
import { Code } from "../../features/code/Code";
import { ContractsContainer } from "../../features/contracts";
import { Logs } from "../../features/logs/components/Logs";
import { runTutorialCheck, runTutorialCheckFx } from "../../features/tutorial-check/model";
import { TutorialText } from "../../features/tutorial/TutorialText";
import { TutorialsPanel } from "../../features/tutorial/TutorialsPanel";
import { $tutorials } from "../../features/tutorial/model";
import {
  $activeComponentTutorial,
  $activeTab,
  $tutorialChecksState,
  TutorialChecksStatus,
  TutorialLayoutComponent,
  changeActiveTab,
  openTutorialText,
  setActiveComponentTutorial,
} from "./model";

const tutorialButton = (
  <Button
    overrides={{
      Root: {
        style: {
          gridColumn: "1 / 3",
          lineHeight: "12px",
          fontWeight: 100,
          fontSize: "16px",
          color: "rgb(189, 189, 189)",
          backgroundColor: COLORS.blue800,
          ":hover": {
            backgroundColor: COLORS.blue700,
          },
        },
      },
    }}
    kind={BUTTON_KIND.secondary}
    size={BUTTON_SIZE.large}
    onClick={() => {
      openTutorialText();
    }}
  >
    Tutorial
  </Button>
);

const TutorialMobileLayout = () => {
  const [activeComponent, runningChecks, tutorialChecks, tutorials, activeKey] = useUnit([
    $activeComponentTutorial,
    runTutorialCheckFx.pending,
    $tutorialChecksState,
    $tutorials,
    $activeTab,
  ]);

  let checkButtonBckgColor: string;
  switch (tutorialChecks) {
    case TutorialChecksStatus.Successful:
      checkButtonBckgColor = COLORS.green200;
      break;
    case TutorialChecksStatus.Failed:
      checkButtonBckgColor = COLORS.red200;
      break;
    case TutorialChecksStatus.Initialized:
      checkButtonBckgColor = COLORS.yellow200;
      break;
    default:
      checkButtonBckgColor = COLORS.gray500;
      break;
  }
  const runCheckButton = (disabled: boolean) => (
    <Button
      kind={BUTTON_KIND.secondary}
      isLoading={runningChecks}
      size={BUTTON_SIZE.default}
      onClick={() => runTutorialCheck()}
      disabled={disabled}
      overrides={{
        BaseButton: {
          style: {
            lineHeight: 1,
            backgroundColor: checkButtonBckgColor,
            color: COLORS.black,
            gridColumn: "1 / 3",
          },
        },
      }}
      data-testid="run-checks-button"
    >
      Run Checks
    </Button>
  );
  const featureMap = new Map<TutorialLayoutComponent, () => JSX.Element>();
  featureMap.set(TutorialLayoutComponent.Code, () => (
    <Code extraMobileButton={tutorialButton} extraToolbarButton={runCheckButton(!tutorialChecks)} />
  ));
  featureMap.set(TutorialLayoutComponent.Logs, Logs);
  featureMap.set(TutorialLayoutComponent.Contracts, ContractsContainer);
  featureMap.set(TutorialLayoutComponent.TutorialText, TutorialText);
  featureMap.set(TutorialLayoutComponent.Tutorials, () => <TutorialsPanel tutorials={tutorials} />);
  const Component = activeComponent ? featureMap.get(activeComponent) : null;
  const handlers = useSwipeable({
    onSwipedLeft: () => setActiveComponentTutorial(TutorialLayoutComponent.Code),
    onSwipedRight: () => setActiveComponentTutorial(TutorialLayoutComponent.Code),
  });

  return (
    <div {...handlers}>
      <Tabs
        onChange={({ activeKey }) => {
          changeActiveTab(activeKey.toString());
          if (activeKey === "0") {
            setActiveComponentTutorial(TutorialLayoutComponent.Code);
          }
          if (activeKey === "1") {
            setActiveComponentTutorial(TutorialLayoutComponent.Tutorials);
          }
        }}
        activeKey={activeKey}
        overrides={{
          Root: {
            style: {
              height: "100%",
              display: "flex",
              flexDirection: "column",
            },
          },
          TabContent: {
            style: {
              height: "100%",
              flex: "1 1 auto",
            },
          },
          TabBar: {
            style: {
              display: "flex",
              justifyContent: "center",
              alignItems: "center",
            },
          },
          Tab: {
            style: {
              flex: 1,
              display: "flex",
              textAlign: "center",
              alignContent: "center",
              justifyContent: "center",
              fontSize: "16px",
              fontWeight: "400",
              width: "50vw",
              ":hover": {
                backgroundColor: COLORS.blue800,
              },
            },
          },
        }}
      >
        <Tab title="Code" key="0">
          <Component />
        </Tab>
        <Tab title="Tutorials" key="1">
          <TutorialsPanel tutorials={tutorials} />
        </Tab>
      </Tabs>
    </div>
  );
};

export { TutorialMobileLayout };
