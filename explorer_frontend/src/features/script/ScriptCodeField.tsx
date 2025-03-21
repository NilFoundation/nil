import { useStyletron } from "baseui";
import { useMobile } from "../shared/hooks/useMobile";
import { useMemo } from "react";
import { Extension } from "@codemirror/state";
import { basicSetup } from "@uiw/react-codemirror";
import { CodeField, CodeFieldProps } from "@nilfoundation/ui-kit";

export const ScriptCodeField = ({ displayCopy = false,
  highlightOnHover = false,
  showLineNumbers = false,
  extensions = [],
  ...rest }: CodeFieldProps) => {

  const [css, theme] = useStyletron();

  const [isMobile] = useMobile();

  const codemirrorExtensions = useMemo<Extension[]>(() => {
    return [
      ...basicSetup({
        closeBrackets: true,
        lineNumbers: !isMobile,
      }),
    ].concat(extensions);
  }, [isMobile, extensions]);

  return (
    <CodeField
      extensions= { codemirrorExtensions }
      displayCopy = { displayCopy }
      highlightOnHover = { highlightOnHover }
      showLineNumbers = { showLineNumbers }
      className = {
        css({
          backgroundColor: `${theme.colors.backgroundPrimary
      }!important`,
      })}
      themeOverrides={{
        settings: {
          lineHighlight: "rgba(255, 255, 255, 0.05)",
          gutterBackground: `${ theme.colors.backgroundPrimary } !important`,
        },
      }}
      {...rest}      
    />
  );
};