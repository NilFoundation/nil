import {
  BUTTON_KIND,
  BUTTON_SIZE,
  ButtonIcon,
  COLORS,
  CopyButton,
  LabelMedium,
  Spinner,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import { HyperlinkIcon, Link, OverflowEllipsis, StatefulPopover, useMobile } from "../../shared";
import {
  $codeSnippetHash,
  $shareCodeSnippetError,
  setCodeSnippetEvent,
  setCodeSnippetFx,
} from "../model";
import { useUnit } from "effector-react";
import type { FC } from "react";
import { sandboxWithHashRoute } from "../../routing";
import { expandProperty } from "inline-style-expand-shorthand";

type HyperlinkButtonProps = {
  disabled?: boolean;
};

export const HyperlinkButton: FC<HyperlinkButtonProps> = ({ disabled }) => {
  const [isMobile] = useMobile();
  const [css] = useStyletron();
  const [shareCodeSnippetPending, codeHash, shareCodeError] = useUnit([
    setCodeSnippetFx.pending,
    $codeSnippetHash,
    $shareCodeSnippetError,
  ]);
  const link = !codeHash ? null : `${window.location.origin}/sandbox/${codeHash}`;

  return (
    <StatefulPopover
      popoverMargin={8}
      content={
        <div
          className={css({
            backgroundColor: COLORS.gray800,
            height: "48px",
            width: isMobile ? "300px" : "400px",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: "16px",
            paddingLeft: "24px",
            paddingRight: "24px",
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
                    to={sandboxWithHashRoute}
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
        })}
        icon={<HyperlinkIcon />}
        kind={BUTTON_KIND.secondary}
        size={isMobile ? BUTTON_SIZE.compact : BUTTON_SIZE.large}
      />
    </StatefulPopover>
  );
};
