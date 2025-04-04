import { CodeField, type CodeFieldProps } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";

export interface CustomCodeFieldProps extends CodeFieldProps {
  isSolidity?: boolean;
}

export const CustomCodeField = ({
  displayCopy = false,
  highlightOnHover = false,
  showLineNumbers = false,
  extensions,
  code,
  ...rest
}: CustomCodeFieldProps) => {
  const [css, theme] = useStyletron();

  return (
    <CodeField
      extensions={extensions}
      editable
      readOnly={false}
      displayCopy={displayCopy}
      highlightOnHover={highlightOnHover}
      showLineNumbers={showLineNumbers}
      code={code}
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
