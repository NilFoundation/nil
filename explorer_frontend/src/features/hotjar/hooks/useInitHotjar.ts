import { useEffect } from "react";
import { getRuntimeConfigOrThrow } from "../../runtime-config";

const { HOTJAR_ID, HOTJAR_VERSION } = getRuntimeConfigOrThrow();

export const useInitHotjar = () => {
  useEffect(() => {
    const isProduction = import.meta.env.PROD;

    if (!HOTJAR_ID || !HOTJAR_VERSION) {
      throw new Error("Hotjar site ID or version is not set");
    }

    if (!isProduction) {
      console.warn("Hotjar is only enabled in production");
      return;
    }

    import("@hotjar/browser").then((module) => {
      module.default.init(Number(HOTJAR_ID), Number(HOTJAR_VERSION));
    });
  }, []);
};
