import type { Extension } from "@codemirror/state";
import { CodeField, type CodeFieldProps } from "@nilfoundation/ui-kit";
import { solidity } from "@replit/codemirror-lang-solidity";
import { basicSetup } from "@uiw/react-codemirror";
import { useStyletron } from "baseui";
import { useMemo } from "react";
import { useMobile } from "..";

export const SolidityCodeField = ({
  displayCopy = false,
  highlightOnHover = false,
  showLineNumbers = false,
  extensions = [],
  ...rest
}: CodeFieldProps) => {
  const [css, theme] = useStyletron();

  const [isMobile] = useMobile();
  const codemirrorExtensions = useMemo<Extension[]>(() => {
    return [
      solidity,
      ...basicSetup({
        lineNumbers: !isMobile,
      }),
    ].concat(extensions);
  }, [isMobile, extensions]);

  return (
    <CodeField
      extensions={codemirrorExtensions}
      displayCopy={displayCopy}
      highlightOnHover={highlightOnHover}
      showLineNumbers={showLineNumbers}
      className={css({
        backgroundColor: `${theme.colors.backgroundPrimary} !important`,
      })}
      themeOverrides={{
        settings: {
          lineHighlight: "rgba(255, 255, 255, 0.05)",
          gutterBackground: `${theme.colors.backgroundPrimary} !important`,
        },
      }}
      {...rest}


    />
  );
};
