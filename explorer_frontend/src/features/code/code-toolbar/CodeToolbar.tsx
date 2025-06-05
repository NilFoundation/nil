import { useStore, useUnit } from "effector-react";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { getMobileStyles } from "../../../styleHelpers.ts";
import { tutorialWithUrlStringRoute } from "../../routing/routes/tutorialRoute.ts";
import { useMobile } from "../../shared/hooks/useMobile.ts";
import { $projectTab, $toolbarLoading, ProjectTab, compileCode, runScript } from "../model.ts";
import { CompileVersionButton } from "./CompileVersionButton.tsx";
import { HyperlinkButton } from "./HyperlinkButton";
import { OpenProjectButton } from "./OpenProjectButton.tsx";
import { ResourcesButton } from "./ResourcesButton.tsx";

type CodeToolbarProps = {
  disabled: boolean;
  isSolidity?: boolean;
};

export const CodeToolbar: FC<CodeToolbarProps> = ({ disabled, isSolidity = false }) => {
  const [css] = useStyletron();
  const isTutorial = useStore(tutorialWithUrlStringRoute.$isOpened);
  const [isMobile] = useMobile();
  const [isLoading, projectTab] = useUnit([$toolbarLoading, $projectTab]);

  const compileButtonContent = projectTab === ProjectTab.code ? "Compile" : "Run";

  const borderRadius = isTutorial ? "8px" : "8px 0 0 8px";

  const buttonStyle = isMobile
    ? {
        whiteSpace: "nowrap",
        lineHeight: 1,
        borderRadius: borderRadius,
        height: "48px",
        width: "100%",
        marginRight: "2px",
      }
    : {
        whiteSpace: "nowrap",
        lineHeight: 1,
        marginLeft: "auto",
        marginRight: "2px",
        borderRadius: borderRadius,
        height: "46px",
      };

  return (
    <div
      className={css({
        display: "flex",
        alignItems: "center",
        justifyContent: "flex-end",
        gap: "8px",
        flexGrow: 1,
        ...getMobileStyles({
          flexDirection: "row-reverse",
          justifyContent: "flex-start",
        }),
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
          onClick={() => {
            projectTab === ProjectTab.code ? compileCode() : runScript();
          }}
          disabled={disabled}
          content={compileButtonContent}
        />
      )}
    </div>
  );
};
