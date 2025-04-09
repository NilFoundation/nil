import {
  BUTTON_KIND,
  BUTTON_SIZE,
  ButtonIcon,
  COLORS,
  CopyButton,
  LabelMedium,
  Spinner,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { expandProperty } from "inline-style-expand-shorthand";
import type { FC } from "react";
import { playgroundWithHashRoute } from "../../routing";
import { HyperlinkIcon, Link, OverflowEllipsis, StatefulPopover, useMobile } from "../../shared";
import {
  $codeSnippetHash,
  $shareCodeSnippetError,
  setCodeSnippetEvent,
  setCodeSnippetFx,
} from "../model";

type HyperlinkButtonProps = {
  disabled?: boolean;
};

export const HyperlinkButton: FC<HyperlinkButtonProps> = ({ disabled }) => {
  const [isMobile] = useMobile();
  const [css, theme] = useStyletron();
  const [shareCodeSnippetPending, codeHash, shareCodeError] = useUnit([
    setCodeSnippetFx.pending,
    $codeSnippetHash,
    $shareCodeSnippetError,
  ]);
  const link = !codeHash ? null : `${window.location.origin}/playground/${codeHash}`;

  return (
    <StatefulPopover
      popoverMargin={8}
      content={
        <div
          className={css({
            height: "46px",
            width: isMobile ? "300px" : "400px",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: "16px",
            paddingLeft: "24px",
            paddingRight: "24px",
            backgroundColor: `${theme.colors.inputButtonAndDropdownOverrideBackgroundColor} !important`,
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
          width: isMobile ? "32px" : "46px",
          height: isMobile ? "32px" : "46px",
          flexShrink: 0,
          backgroundColor: `${theme.colors.inputButtonAndDropdownOverrideBackgroundColor} !important`,
          ":hover": {
            backgroundColor: `${theme.colors.inputButtonAndDropdownOverrideBackgroundHoverColor} !important`,
          },
        })}
        icon={<HyperlinkIcon />}
        kind={BUTTON_KIND.secondary}
        size={isMobile ? BUTTON_SIZE.compact : BUTTON_SIZE.large}
      />
    </StatefulPopover>
  );
};
