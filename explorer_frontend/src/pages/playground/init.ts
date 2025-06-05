import { persist } from "effector-storage/session";
import {
  clickOnBackButton,
  clickOnContractsButton,
  clickOnLogButton,
} from "../../features/code/model";
import { $activeComponent, $activeTab, LayoutComponent, setActiveTab } from "./model";

$activeComponent.on(clickOnLogButton, () => LayoutComponent.Logs);
$activeComponent.on(clickOnContractsButton, () => LayoutComponent.Contracts);
$activeComponent.on(clickOnBackButton, () => LayoutComponent.Code);

persist({
  store: $activeComponent,
  key: "activeComponentPlayground",
});

persist({
  store: $activeTab,
  key: "activeTab",
});

$activeTab.on(setActiveTab, (_, tab) => tab);
