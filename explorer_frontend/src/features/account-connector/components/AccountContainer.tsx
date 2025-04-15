import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { useSwipeable } from "react-swipeable";
import { ActiveComponent } from "../ActiveComponent";
import { $activeComponent, setActiveComponent } from "../model";
import { AccountRegenerationScreen } from "./AccountRegenerationScreen.tsx";
import { InfoAndBalancesScreen } from "./InfoAndBalancesScreen.tsx";
import { MainScreen } from "./MainScreen";
import { PrivateKeyScreen } from "./PrivateKeyScreen.tsx";
import { RpcUrlScreen } from "./RpcUrlScreen.tsx";
import { SettingsScreen } from "./SettingsScreen.tsx";
import { TopUpPanel } from "./TopUpPanel";

const featureMap = new Map();
featureMap.set(ActiveComponent.InfoAndBalances, InfoAndBalancesScreen);
featureMap.set(ActiveComponent.Main, MainScreen);
featureMap.set(ActiveComponent.Topup, TopUpPanel);
featureMap.set(ActiveComponent.SettingsScreen, SettingsScreen);
featureMap.set(ActiveComponent.PrivateKeyScreen, PrivateKeyScreen);
featureMap.set(ActiveComponent.AccountRegenerationScreen, AccountRegenerationScreen);
featureMap.set(ActiveComponent.RpcUrlScreen, RpcUrlScreen);

const AccountContainer = () => {
  const activeComponent = useUnit($activeComponent);
  const Component = activeComponent ? featureMap.get(activeComponent) : null;
  const [css, theme] = useStyletron();
  const handlers = useSwipeable({
    onSwipedLeft: () => setActiveComponent(ActiveComponent.InfoAndBalances),
    onSwipedRight: () => setActiveComponent(ActiveComponent.InfoAndBalances),
  });

  return (
    <div
      {...handlers}
      className={css({
        ...expandProperty("padding", "24px"),
        ...expandProperty("borderRadius", "16px"),
        width: "100%",
        maxWidth: "420px",
        backgroundColor: theme.colors.backgroundSecondary,
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
