import { BUTTON_KIND, BUTTON_SIZE, Button } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useStore } from "effector-react";
import type { FC } from "react";
import { tutorialWithUrlStringRoute } from "../../routing/routes/tutorialRoute";
import { useMobile } from "../../shared/hooks/useMobile";
import { CompilerVersionButton } from "./CompilerVersionButton";

type CompileVersionButtonProps = {
  isLoading: boolean;
  onClick: any;
  disabled: boolean;
  content: JSX.Element | string;
};

export const CompileVersionButton: FC<CompileVersionButtonProps> = ({
  isLoading,
  onClick,
  disabled,
  content,
}) => {
  const [css, theme] = useStyletron();
  const isTutorial = useStore(tutorialWithUrlStringRoute.$isOpened);
  const borderRadius = isTutorial ? "8px" : "8px 0 0 8px";
  const [isMobile] = useMobile();
  const style = isMobile
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

  let containerStyle = {
    display: "flex",
  };
  if (isMobile && isTutorial) {
    containerStyle = {
      display: "grid",
      gridColumn: "1 / 2",
      width: "100%",
    };
  } else if (isMobile) {
    containerStyle = {
      display: "flex",
      gridColumn: "1 / 3",
      width: "100%",
    };
  }

  return (
    <div className={css(containerStyle)}>
      <Button
        kind={BUTTON_KIND.primary}
        isLoading={isLoading}
        size={BUTTON_SIZE.default}
        onClick={onClick}
        disabled={disabled}
        overrides={{
          Root: {
            style: style,
          },
        }}
        data-testid="compile-button"
      >
        {content}
      </Button>
      {!isTutorial && (
        <>
          <CompilerVersionButton disabled={disabled} isMobile={isMobile} />{" "}
        </>
      )}
    </div>
  );
};
