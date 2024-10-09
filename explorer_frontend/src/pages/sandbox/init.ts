import { $activeComponent, setActiveComponent } from "./model";
import { persist } from "effector-storage/session";

$activeComponent.on(setActiveComponent, (_, payload) => payload);

persist({
  store: $activeComponent,
  key: "activeComponentSandbox",
});
