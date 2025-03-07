import { COLORS } from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { useSwipeable } from "react-swipeable";
import { useStyletron } from "styletron-react";
import { ActiveComponent } from "../ActiveComponent";
import { $activeComponent, setActiveComponent } from "../model";
import { MainScreen } from "./MainScreen";
import { RpcUrlScreen } from "./RpcUrlScreen.tsx";
import { TopUpPanel } from "./TopUpPanel";
import { isTutorialPage } from "../../code/model.ts";
import { is } from "effector";

const featureMap = new Map();
featureMap.set(ActiveComponent.RpcUrl, RpcUrlScreen);
featureMap.set(ActiveComponent.Main, MainScreen);
featureMap.set(ActiveComponent.Topup, TopUpPanel);

const AccountContainer = () => {
  const [activeComponent, isTutorial] = useUnit([$activeComponent, isTutorialPage]);
  const Component = activeComponent ? featureMap.get(activeComponent) : null;
  const [css] = useStyletron();
  const handlers = useSwipeable({
    onSwipedLeft: () => setActiveComponent(ActiveComponent.RpcUrl),
    onSwipedRight: () => setActiveComponent(ActiveComponent.RpcUrl),
  });

  return (
    <div
      {...handlers}
      className={css({
        ...expandProperty("padding", "24px"),
        ...expandProperty("borderRadius", "16px"),
        width: "100%",
        maxWidth: "420px",
        backgroundColor: isTutorial ? COLORS.blue900 : COLORS.gray900,
        "@media (min-width: 421px)": {
          width: "420px",
          margin: "0 auto",
        },
      })}
    >
      <Component />
    </div>
  );
};

export { AccountContainer };
