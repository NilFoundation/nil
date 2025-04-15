import {
  ArrowUpIcon,
  BUTTON_KIND,
  BUTTON_SIZE,
  Button,
  COLORS,
  LabelMedium,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { FC } from "react";

type BackLinkProps = {
  title: string;
  goBackCb?: () => void;
  disabled?: boolean;
};

const BackLink: FC<BackLinkProps> = ({ title, goBackCb, disabled }) => {
  const [css, theme] = useStyletron();

  return (
    <div
      className={css({
        width: "100%",
        height: "100%",
        display: "flex",
        gap: "12px",
        alignItems: "center",
      })}
    >
      <Button
        className={css({
          width: "32px",
          height: "32px",
        })}
        overrides={{
          Root: {
            style: {
              paddingLeft: 0,
              paddingRight: 0,
              backgroundColor: theme.colors.backLinkBackgroundColor,
              ":hover": {
                backgroundColor: theme.colors.backLinkBackgroundHoverColor,
              },
            },
          },
        }}
        kind={BUTTON_KIND.secondary}
        size={BUTTON_SIZE.compact}
        onClick={goBackCb}
        disabled={disabled}
      >
        <ArrowUpIcon
          size={12}
          className={css({
            transform: "rotate(-90deg)",
          })}
        />
      </Button>
      <LabelMedium color={COLORS.gray50}>{title}</LabelMedium>
    </div>
  );
};

export { BackLink };
