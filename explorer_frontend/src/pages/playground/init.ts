import { persist } from "effector-storage/session";
import {
  userClickOnBackButton,
  userClickOnContractsButton,
  userClickOnLogButton,
} from "../../features/code/model";
import { $activeComponent, LayoutComponent } from "./model";

$activeComponent.on(userClickOnLogButton, (_) => LayoutComponent.Logs);
$activeComponent.on(userClickOnContractsButton, (_) => LayoutComponent.Contracts);
$activeComponent.on(userClickOnBackButton, (_) => LayoutComponent.Code);

persist({
  store: $activeComponent,
  key: "activeComponentPlayground",
});
