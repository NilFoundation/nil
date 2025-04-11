import { createEvent, createStore } from "effector";

export enum LayoutComponent {
  Code = "0",
  Contracts = "1",
  Logs = "2",
  Script = "3"
}

export const $activeComponent = createStore<LayoutComponent>(LayoutComponent.Code);

export const setActiveComponent = createEvent<LayoutComponent>();
