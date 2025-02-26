import { useUnit } from "effector-react";
import { useSwipeable } from "react-swipeable";
import { Code } from "../../features/code/Code";
import { ContractsContainer } from "../../features/contracts";
import { Logs } from "../../features/logs/components/Logs";
import { TutorialText } from "../../features/tutorial/TutorialText";
import {
  $activeComponentTutorial,
  TutorialLayoutComponent,
  setActiveComponentTutorial,
} from "./model";

const featureMap = new Map();
featureMap.set(TutorialLayoutComponent.Code, Code);
featureMap.set(TutorialLayoutComponent.Logs, Logs);
featureMap.set(TutorialLayoutComponent.Contracts, ContractsContainer);
featureMap.set(TutorialLayoutComponent.TutorialText, TutorialText);

const TutorialMobileLayout = () => {
  const activeComponent = useUnit($activeComponentTutorial);
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
