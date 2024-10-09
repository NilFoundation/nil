import { useEffect } from "react";

export const useInitHotjar = () => {
  useEffect(() => {
    const siteId = import.meta.env.VITE_HOTJAR_ID;
    const hotjarVersion = import.meta.env.VITE_HOTJAR_VERSION;
    const isProduction = import.meta.env.PROD;

    if (!siteId || !hotjarVersion) {
      throw new Error("Hotjar site ID or version is not set");
    }

    if (!isProduction) {
      console.warn("Hotjar is only enabled in production");
      return;
    }

    import("@hotjar/browser").then((module) => {
      module.default.init(Number(siteId), Number(hotjarVersion));
    });
  }, []);
};
