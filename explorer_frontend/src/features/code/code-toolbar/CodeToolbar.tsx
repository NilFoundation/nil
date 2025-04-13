import type { FC, ReactNode } from "react";
import { useStyletron } from "styletron-react";
import { useMobile } from "../../shared";
import { CompilerVersionButton } from "./CompilerVersionButton.tsx";
import { HyperlinkButton } from "./HyperlinkButton";
import { OpenProjectButton } from "./OpenProjectButton.tsx";
import { QuestionButton } from "./QuestionButton";

type CodeToolbarProps = {
  disabled: boolean;
  extraToolbarButton?: ReactNode;
};

export const CodeToolbar: FC<CodeToolbarProps> = ({ disabled, extraToolbarButton }) => {
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
      <QuestionButton />
      <HyperlinkButton disabled={disabled} />
      {extraToolbarButton === undefined && (
        <>
          {" "}
          <OpenProjectButton disabled={disabled} /> <CompilerVersionButton
            disabled={disabled}
          />{" "}
        </>
      )}
    </div>
  );
};
