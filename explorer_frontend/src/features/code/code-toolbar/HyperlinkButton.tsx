import {
  BUTTON_KIND,
  BUTTON_SIZE,
  ButtonIcon,
  COLORS,
  CopyButton,
  LabelMedium,
  Spinner,
} from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import { expandProperty } from "inline-style-expand-shorthand";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { playgroundWithHashRoute } from "../../routing";
import { HyperlinkIcon, Link, OverflowEllipsis, StatefulPopover, useMobile } from "../../shared";
import {
  $codeSnippetHash,
  $shareCodeSnippetError,
  isTutorialPage,
  setCodeSnippetEvent,
  setCodeSnippetFx,
} from "../model";

type HyperlinkButtonProps = {
  disabled?: boolean;
};

export const HyperlinkButton: FC<HyperlinkButtonProps> = ({ disabled }) => {
  const [isMobile] = useMobile();
  const [css] = useStyletron();
  const [shareCodeSnippetPending, codeHash, shareCodeError, isTutorial] = useUnit([
    setCodeSnippetFx.pending,
    $codeSnippetHash,
    $shareCodeSnippetError,
    isTutorialPage
  ]);
  const link = !codeHash ? null : `${window.location.origin}/playground/${codeHash}`;

  return (
    <StatefulPopover
      popoverMargin={8}
      content={
        <div
          className={css({
            height: "48px",
            width: isMobile ? "300px" : "400px",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: "16px",
            paddingLeft: "24px",
            paddingRight: "24px",
            backgroundColor: isTutorial ? COLORS.blue800 : COLORS.gray800,

            ...expandProperty("borderRadius", "8px"),
          })}
        >
          {!!link && !shareCodeError && (
            <>
              <div
                className={css({
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "flex-start",
                  gap: "1ch",
                  width: "calc(100% - 40px)",
                })}
              >
                <LabelMedium
                  className={css({
                    width: "100%",
                  })}
                >
                  <Link
                    to={playgroundWithHashRoute}
                    params={{ snippetHash: codeHash }}
                    target="_blank"
                  >
                    <OverflowEllipsis>{link}</OverflowEllipsis>
                  </Link>
                </LabelMedium>
              </div>
              <CopyButton textToCopy={link} />
            </>
          )}
          {shareCodeError && (
            <LabelMedium color={COLORS.red200}>
              An error occurred while generating the link
            </LabelMedium>
          )}
          {shareCodeSnippetPending && (
            <div
              className={css({
                height: "100%",
                width: "100%",
                display: "flex",
                justifyContent: "center",
                alignItems: "center",
              })}
            >
              <Spinner />
            </div>
          )}
        </div>
      }
      placement="bottom"
      autoFocus
      onOpen={() => setCodeSnippetEvent()}
    >
      <ButtonIcon
        disabled={disabled}
        className={css({
          width: isMobile ? "32px" : "48px",
          height: isMobile ? "32px" : "48px",
          flexShrink: 0,
          backgroundColor: isTutorial ? `${COLORS.blue800} !important` : COLORS.gray800,
          ':hover': { backgroundColor: isTutorial ? `${COLORS.blue700} !important` : COLORS.gray700 },
        })}
        icon={<HyperlinkIcon />}
        kind={BUTTON_KIND.secondary}
        size={isMobile ? BUTTON_SIZE.compact : BUTTON_SIZE.large}
      />
    </StatefulPopover>
  );
};
