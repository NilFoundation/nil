import type { Extension } from "@codemirror/state";
import { CodeField, COLORS, type CodeFieldProps } from "@nilfoundation/ui-kit";
import { solidity } from "@replit/codemirror-lang-solidity";
import { basicSetup } from "@uiw/react-codemirror";
import { useMemo } from "react";
import { useMobile } from "..";
import { isTutorialPage } from "../../code/model";
import { useUnit } from "effector-react";
import { useStyletron } from "styletron-react";

export const SolidityCodeField = ({
  displayCopy = false,
  highlightOnHover = false,
  showLineNumbers = false,
  extensions = [],
  ...rest
}: CodeFieldProps) => {
  const [css] = useStyletron();

  const isTutorial = useUnit(isTutorialPage);
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
        backgroundColor: isTutorial ? `${COLORS.blue900} !important` : COLORS.gray900,
      })}
      themeOverrides={{
        settings: {
          lineHighlight: "rgba(255, 255, 255, 0.05)",
          gutterBackground: isTutorial ? `${COLORS.blue900} !important` : COLORS.gray900,
        },
      }}
      {...rest}
    />
  );
};
