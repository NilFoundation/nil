import { useUnit } from "effector-react";
import { $activeComponent, LayoutComponent, setActiveComponent } from "./model";
import { Code } from "../../features/code/Code";
import { Logs } from "../../features/logs/Logs";
import { Contracts } from "../../features/contracts/Contracts";
import { useSwipeable } from "react-swipeable";

const featureMap = new Map();
featureMap.set(LayoutComponent.Code, Code);
featureMap.set(LayoutComponent.Logs, Logs);
featureMap.set(LayoutComponent.Contracts, Contracts);

const SandboxMobileLayout = () => {
  const activeComponent = useUnit($activeComponent);
  const Component = activeComponent ? featureMap.get(activeComponent) : null;
  const handlers = useSwipeable({
    onSwipedLeft: () => setActiveComponent(LayoutComponent.Code),
    onSwipedRight: () => setActiveComponent(LayoutComponent.Code),
  });

  return (
    <div {...handlers}>
      <Component />
    </div>
  );
};

export { SandboxMobileLayout };
