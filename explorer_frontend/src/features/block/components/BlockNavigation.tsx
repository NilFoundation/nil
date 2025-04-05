import type { BlockListElement } from "@nilfoundation/explorer-backend/daos/blocks";
import {
  BUTTON_KIND,
  ButtonIcon,
  COLORS,
  ChevronLeftIcon,
  ChevronRightIcon,
} from "@nilfoundation/ui-kit";
import { Link } from "atomic-router-react";
import { type Theme, useStyletron } from "baseui";
import { ParagraphSmall } from "baseui/typography";
import type { ReactElement } from "react";
import { blockRoute } from "../../routing";

export const BlockNavigation = ({
  blockInfo,
}: {
  blockInfo: BlockListElement;
}) => {
  const [css, theme] = useStyletron();

  if (blockInfo) {
    return (
      <div
        className={css({
          display: "flex",
          flexDirection: "row",
          alignItems: "center",
          gap: "1rem",
          marginBlockStart: "-.7rem",
        })}
      >
        <ParagraphSmall
          color={COLORS.gray100}
          className={css({
            display: "inline-block",
            wordBreak: "break-all",
          })}
        >
          {blockInfo.id}
        </ParagraphSmall>
        <div
          className={css({
            display: "flex",
            flexDirection: "row",
            gap: ".25rem",
          })}
        >
          {+blockInfo.id > 0 ? (
            <Link
              to={blockRoute}
              params={{
                shard: blockInfo.shard_id.toString(),
                id: (+blockInfo.id - 1).toString(),
              }}
            >
              <ButtonIconOverride theme={theme} icon={<ChevronLeftIcon />} />
            </Link>
          ) : null}
          <Link
            to={blockRoute}
            params={{
              shard: blockInfo.shard_id.toString(),
              id: (+blockInfo.id + 1).toString(),
            }}
          >
            <ButtonIconOverride theme={theme} icon={<ChevronRightIcon />} />
          </Link>
        </div>
      </div>
    );
  }
};

const ButtonIconOverride = ({
  icon,
  theme,
}: {
  icon: ReactElement;
  theme: Theme;
}) => {
  return (
    <ButtonIcon
      kind={BUTTON_KIND.tertiary}
      icon={icon}
      overrides={{
        Root: {
          style: {
            height: "2rem",
            width: "2rem",
            backgroundColor: theme.colors.backgroundSecondary,
            ":hover": {
              backgroundColor: theme.colors.backgroundTertiary,
            },
          },
        },
      }}
    />
  );
};
