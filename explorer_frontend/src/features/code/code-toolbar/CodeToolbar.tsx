import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { QuestionButton } from "./QuestionButton";
import { HyperlinkButton } from "./HyperlinkButton";
import { ExamplesButton } from "./ExamplesButton";
import { BackRouterNavigationButton, useMobile } from "../../shared";
import { CompilerVersionButton } from "./CompilerVersionButton.tsx";

type CodeToolbarProps = {
  disabled: boolean;
};

export const CodeToolbar: FC<CodeToolbarProps> = ({ disabled }) => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  return (
    <div
      className={css({
        display: "flex",
        alignItems: "center",
        justifyContent: isMobile ? "flex-end" : "flex-start",
        gap: "8px",
        flexGrow: 1,
      })}
    >
      {!isMobile && (
        <BackRouterNavigationButton
          overrides={{
            Root: {
              style: {
                marginRight: "auto",
              },
            },
          }}
        />
      )}
      <QuestionButton />
      <HyperlinkButton disabled={disabled} />
      <ExamplesButton disabled={disabled} />
      <CompilerVersionButton />
    </div>
  );
};
