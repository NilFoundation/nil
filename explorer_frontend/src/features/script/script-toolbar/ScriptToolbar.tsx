import { FC } from "react";
import { useStyletron } from "styletron-react";
import { useMobile } from "../../shared/hooks/useMobile";
import { BackRouterNavigationButton } from "../../shared/components/BackRouterNavigationButton";
import { QuestionButton } from "../../code/code-toolbar/QuestionButton";
import { HyperlinkButton } from "../../code/code-toolbar/HyperlinkButton";

type ScriptToolbarProps = {
  disabled: boolean;
}

export const ScriptToolbar: FC<ScriptToolbarProps> = ({ disabled }) => {
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

    </div>
  );
};