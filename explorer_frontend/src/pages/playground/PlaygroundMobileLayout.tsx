import { useUnit } from "effector-react";
import { useSwipeable } from "react-swipeable";
import { useStyletron } from "styletron-react";
import { Code } from "../../features/code/Code";
import { ContractsContainer } from "../../features/contracts";
import { Logs } from "../../features/logs/components/Logs";
import { $activeComponent, LayoutComponent, setActiveComponent } from "./model";

const featureMap = new Map();
featureMap.set(LayoutComponent.Code, Code);
featureMap.set(LayoutComponent.Logs, Logs);
featureMap.set(LayoutComponent.Contracts, ContractsContainer);

const PlaygroundMobileLayout = () => {
  const [css] = useStyletron();
  const activeComponent = useUnit($activeComponent);
  const Component = activeComponent ? featureMap.get(activeComponent) : null;
  const handlers = useSwipeable({
    onSwipedLeft: () => setActiveComponent(LayoutComponent.Code),
    onSwipedRight: () => setActiveComponent(LayoutComponent.Code),
  });

  return (
    <div
      {...handlers}
      className={css({
        height: "100%",
        width: "100%",
        overflow: "hidden",
      })}
    >
      <Component />
    </div>
  );
};

export { PlaygroundMobileLayout };
