import { useMemo } from "react";
import { useHotkeys } from "react-hotkeys-hook";
import { type StyleObject, useStyletron } from "styletron-react";
import { compileCode } from "../model";

const getOsName = () => {
  const userAgent = window.navigator.userAgent;

  let os = "";

  if (/Windows NT/.test(userAgent)) {
    os = "windows";
  } else if (/Macintosh/.test(userAgent)) {
    os = "mac";
  } else if (/Linux/.test(userAgent) && !/Android/.test(userAgent)) {
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

const getCompileBtnContent = (css: (style: StyleObject) => string) => {
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

const getScriptBtnContent = (css: (style: StyleObject) => string) => {
  switch (os) {
    case "mac":
      return (
        <>
          Run script ⌘ +{" "}
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
    case "windows":
      return (
        <>
          Run script Ctrl +{" "}
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
      return "Run script";
  }
};

export const useCompileButton = () => {
  const [css] = useStyletron();

  const hotKey = os === "mac" ? "Meta+Enter" : "Ctrl+Enter";
  const btnContent = useMemo(() => getCompileBtnContent(css), [css]);

  useHotkeys(
    hotKey,
    () => compileCode(),
    {
      preventDefault: true,
      enableOnContentEditable: true,
    },
    [],
  );

  return btnContent;
};

export const useRunScriptButton = () => {
  const [css] = useStyletron();

  const hotKey = os === "mac" ? "Meta+Enter" : "Ctrl+Enter";
  const btnContent = useMemo(() => getScriptBtnContent(css), [css]);

  useHotkeys(
    hotKey,
    () => compileCode(),
    {
      preventDefault: true,
      enableOnContentEditable: true,
    },
    [],
  );

  return btnContent;
};
