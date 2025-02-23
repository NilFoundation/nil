import { WindowNilProxy } from "./WindowNilProxy.ts";

declare global {
  interface Window {
    nil?: WindowNilProxy;
  }
}

const networkProvider = new WindowNilProxy();
window.nil = networkProvider;
