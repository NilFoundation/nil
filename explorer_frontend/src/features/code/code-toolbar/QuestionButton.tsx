import {
  BUTTON_KIND,
  BUTTON_SIZE,
  ButtonIcon,
  COLORS,
  type Items,
  MENU_SIZE,
  Menu,
} from "@nilfoundation/ui-kit";
import type { MenuOverrides } from "baseui/menu";
import { useStyletron } from "styletron-react";
import { getRuntimeConfigOrThrow } from "../../runtime-config";
import { ArrowUpRightIcon, QuestionIcon, StatefulPopover, useMobile } from "../../shared";
import { isTutorialPage } from "../model";
import { useUnit } from "effector-react";


export const QuestionButton = () => {
  const isTutorial = useUnit(isTutorialPage);
  const menuOverrides: MenuOverrides = {
    List: {
      style: {
        backgroundColor: isTutorial ? COLORS.blue800 : COLORS.gray800,

      },
    },
  };
  const [css] = useStyletron();
  const [isMobile] = useMobile();
  const { PLAYGROUND_FEEDBACK_URL, PLAYGROUND_SUPPORT_URL, PLAYGROUND_DOCS_URL } =
    getRuntimeConfigOrThrow();

  const items: Items = [
    {
      label: "Documentation",
      endEnhancer: <ArrowUpRightIcon />,
      href: PLAYGROUND_DOCS_URL,
    },
    {
      label: "Support",
      endEnhancer: <ArrowUpRightIcon />,
      href: PLAYGROUND_SUPPORT_URL,
    },
    {
      label: (
        <span
          className={css({
            whiteSpace: "nowrap",
          })}
        >
          {"Share feedback"}
        </span>
      ),
      href: PLAYGROUND_FEEDBACK_URL,
    },
  ];

  return (
    <StatefulPopover
      popoverMargin={8}
      content={<Menu
        isDropdown
        items={items}
        size={MENU_SIZE.small}
        overrides={menuOverrides}
      />}
      placement="bottomRight"
      autoFocus
      triggerType="click"
    >
      <ButtonIcon
        className={css({
          width: isMobile ? "32px" : "48px",
          height: isMobile ? "32px" : "48px",
          flexShrink: 0,
        })}
        icon={<QuestionIcon />}
        kind={BUTTON_KIND.secondary}
        size={isMobile ? BUTTON_SIZE.compact : BUTTON_SIZE.large}
        overrides={{
          Root: {
            style: {
              ...(isTutorial ? {
                backgroundColor: COLORS.blue800,
                ':hover': {
                  backgroundColor: COLORS.blue700,
                }
              } : {}
              )
            }
          }
        }}
      />
    </StatefulPopover>
  );
};
