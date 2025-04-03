import type { Extension } from "@codemirror/state";
import type { CodeFieldProps } from "@nilfoundation/ui-kit";
import { solidity } from "@replit/codemirror-lang-solidity";
import { basicSetup } from "@uiw/react-codemirror";
import { useStyletron } from "baseui";
import { useMemo } from "react";
import { useMobile } from "..";
import { javascriptLanguage } from "@codemirror/lang-javascript";

type ExtendedCodeFieldProps = CodeFieldProps & {
  isSolidity?: boolean;
};

export const CodeField = ({
  displayCopy = false,
  highlightOnHover = false,
  showLineNumbers = false,
  extensions = [],
  ...rest
}: ExtendedCodeFieldProps) => {
  const [css, theme] = useStyletron();

  const [isMobile] = useMobile();

  const codemirrorExtensions = useMemo<Extension[]>(() => {
    if (rest.isSolidity) {
      return [
        javascriptLanguage.configure({ dialect: "ts" }, "typescript"),
        ...basicSetup({
          lineNumbers: !isMobile,
        }),
      ].concat(extensions);
    }
    return [
      solidity,
      ...basicSetup({
        lineNumbers: !isMobile,
      }),
    ].concat(extensions);
  }, [isMobile, extensions, rest.isSolidity]);

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
