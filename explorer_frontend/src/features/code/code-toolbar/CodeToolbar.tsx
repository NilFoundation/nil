import { useUnit } from "effector-react";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { BackRouterNavigationButton, useMobile } from "../../shared";
import { isTutorialPage } from "../model.ts";
import { CompilerVersionButton } from "./CompilerVersionButton.tsx";
import { HyperlinkButton } from "./HyperlinkButton";
import { OpenProjectButton } from "./OpenProjectButton.tsx";
import { QuestionButton } from "./QuestionButton";

type CodeToolbarProps = {
  disabled: boolean;
};

export const CodeToolbar: FC<CodeToolbarProps> = ({ disabled }) => {
  const isTutorial = useUnit(isTutorialPage);
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
      <OpenProjectButton disabled={disabled} />
      {!isTutorial && <CompilerVersionButton disabled={disabled} />}
    </div>
  );
};
