import { BUTTON_KIND, BUTTON_SIZE, COLORS } from "@nilfoundation/ui-kit";
import { Button } from "baseui/button";
import { useUnit } from "effector-react";
import { useSwipeable } from "react-swipeable";
import { Code } from "../../features/code/Code";
import { ContractsContainer } from "../../features/contracts";
import { Logs } from "../../features/logs/components/Logs";
import { runTutorialCheck, runTutorialCheckFx } from "../../features/tutorial-check/model";
import { TutorialText } from "../../features/tutorial/TutorialText";
import {
  $activeComponentTutorial,
  $tutorialChecksState,
  TutorialChecksStatus,
  TutorialLayoutComponent,
  setActiveComponentTutorial,
  сlickOnTutorialButton,
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
        },
      },
    }}
    kind={BUTTON_KIND.secondary}
    size={BUTTON_SIZE.large}
    onClick={() => {
      сlickOnTutorialButton();
    }}
  >
    Tutorial
  </Button>
);

const TutorialMobileLayout = () => {
  const [activeComponent, runningChecks, tutorialChecks] = useUnit([
    $activeComponentTutorial,
    runTutorialCheckFx.pending,
    $tutorialChecksState,
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
      checkButtonBckgColor = COLORS.black;
      break;
  }
  const runCheckButton = (
    <Button
      kind={BUTTON_KIND.secondary}
      isLoading={runningChecks}
      size={BUTTON_SIZE.default}
      onClick={() => runTutorialCheck()}
      disabled={!tutorialChecks}
      overrides={{
        Root: {
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
    <Code extraMobileTutorialButton={tutorialButton} runCheckButton={runCheckButton} />
  ));
  featureMap.set(TutorialLayoutComponent.Logs, Logs);
  featureMap.set(TutorialLayoutComponent.Contracts, ContractsContainer);
  featureMap.set(TutorialLayoutComponent.TutorialText, TutorialText);
  const Component = activeComponent ? featureMap.get(activeComponent) : null;
  const handlers = useSwipeable({
    onSwipedLeft: () => setActiveComponentTutorial(TutorialLayoutComponent.Code),
    onSwipedRight: () => setActiveComponentTutorial(TutorialLayoutComponent.Code),
  });

  return (
    <div {...handlers}>
      <Component />
    </div>
  );
};

export { TutorialMobileLayout };
