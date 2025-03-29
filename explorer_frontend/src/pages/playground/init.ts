import { persist } from "effector-storage/session";
import {
  clickOnBackButton,
  clickOnContractsButton,
  clickOnLogButton,
} from "../../features/code/model";
import { $activeComponent, LayoutComponent } from "./model";

$activeComponent.on(clickOnLogButton, () => LayoutComponent.Logs);
$activeComponent.on(clickOnContractsButton, () => LayoutComponent.Contracts);
$activeComponent.on(clickOnBackButton, () => LayoutComponent.Code);

persist({
  store: $activeComponent,
  key: "activeComponentPlayground",
});
