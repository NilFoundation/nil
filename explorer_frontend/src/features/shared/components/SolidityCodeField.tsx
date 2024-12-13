import { CodeField, type CodeFieldProps } from "@nilfoundation/ui-kit";
import { solidity } from "@replit/codemirror-lang-solidity";
import { basicSetup } from "@uiw/react-codemirror";
import { useMobile } from "..";
import { useMemo } from "react";
import type { Extension } from "@codemirror/state";

export const SolidityCodeField = ({
  displayCopy = false,
  highlightOnHover = false,
  showLineNumbers = false,
  extensions,
  ...rest
}: CodeFieldProps) => {
  const [isMobile] = useMobile();
  const codemirrorExtensions = useMemo<Extension[]>(() => {
    return [
      solidity,
      ...basicSetup({
        lineNumbers: !isMobile,
      }),
      ...extensions ?? [],
    ];
  }, [isMobile, extensions]);

  return (
    <CodeField
      extensions={codemirrorExtensions}
      displayCopy={displayCopy}
      highlightOnHover={highlightOnHover}
      showLineNumbers={showLineNumbers}
      themeOverrides={{
        settings: {
          lineHighlight: "rgba(255, 255, 255, 0.05)",
        },
      }}
      {...rest}
    />
  );
};
