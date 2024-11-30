import { useHotkeys } from "react-hotkeys-hook";
import { compile } from "../model";
import { type StyleObject, useStyletron } from "styletron-react";
import { useMemo } from "react";

const getOsName = () => {
  const userAgent = window.navigator.userAgent;

  let os = "";

  if (userAgent.indexOf("Win") !== -1) {
    os = "windows";
  } else if (userAgent.indexOf("Mac") !== -1) {
    os = "mac";
  } else if (userAgent.indexOf("X11") !== -1) {
    os = "linux";
  } else if (/Android/.test(userAgent)) {
    os = "android";
  } else if (/iPhone|iPad|iPod/.test(userAgent)) {
    os = "ios";
  } else {
    os = "Unknown";
  }

  return os;
};

const os = getOsName();

const getBtnContent = (css: (style: StyleObject) => string) => {
  switch (os) {
    case "mac":
      return (
        <>
          Compile ⌘ +{" "}
          <span
            className={css({
              marginLeft: "0.5ch",
              paddingTop: "2px",
            })}
          >
            ↵
          </span>
        </>
      );
    case "linux":
    case "windows":
      return (
        <>
          Compile Ctrl +{" "}
          <span
            className={css({
              marginLeft: "0.5ch",
              paddingTop: "2px",
            })}
          >
            ↵
          </span>
        </>
      );
    default:
      return "Compile";
  }
};

export const useCompileButton = () => {
  const [css] = useStyletron();

  const hotKey = os === "windows" ? "Ctrl+Enter" : os === "mac" ? "Meta+Enter" : "Ctrl+Enter";
  const btnContent = useMemo(() => getBtnContent(css), [css]);

  useHotkeys(
    hotKey,
    () => compile(),
    {
      preventDefault: true,
      enableOnContentEditable: true,
    },
    [],
  );

  return btnContent;
};
