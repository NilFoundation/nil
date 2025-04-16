import { useStore } from "effector-react";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { tutorialWithUrlStringRoute } from "../../routing/routes/tutorialRoute.ts";
import { useMobile } from "../../shared/hooks/useMobile.ts";
import { CompileVersionButton } from "./CompileVersionButton.tsx";
import { HyperlinkButton } from "./HyperlinkButton";
import { OpenProjectButton } from "./OpenProjectButton.tsx";
import { ResourcesButton } from "./ResourcesButton.tsx";

type CodeToolbarProps = {
  disabled: boolean;
  isLoading: boolean;
  onCompileButtonClick: any;
  compileButtonContent: JSX.Element | string;
};

export const CodeToolbar: FC<CodeToolbarProps> = ({
  disabled,
  isLoading,
  onCompileButtonClick,
  compileButtonContent,
}) => {
  const [css] = useStyletron();
  const isTutorial = useStore(tutorialWithUrlStringRoute.$isOpened);
  const [isMobile] = useMobile();
  return (
    <div
      className={css({
        display: "flex",
        alignItems: "center",
        justifyContent: "flex-end",
        gap: "8px",
        flexGrow: 1,
      })}
    >
      <ResourcesButton />
      <HyperlinkButton disabled={disabled} />
      {!isTutorial && (
        <>
          {" "}
          <OpenProjectButton disabled={disabled} />
        </>
      )}
      {!isMobile && (
        <CompileVersionButton
          isLoading={isLoading}
          onClick={onCompileButtonClick}
          disabled={disabled}
          content={compileButtonContent}
          isTutorial={isTutorial}
        />
      )}
    </div>
  );
};
