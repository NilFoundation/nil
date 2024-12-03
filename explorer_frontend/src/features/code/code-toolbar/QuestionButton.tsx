import {
  BUTTON_KIND,
  BUTTON_SIZE,
  ButtonIcon,
  type Items,
  MENU_SIZE,
  Menu,
  COLORS,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import { ArrowUpRightIcon, QuestionIcon, StatefulPopover, useMobile } from "../../shared";
import type { MenuOverrides } from "baseui/menu";

export const QuestionButton = () => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  return (
    <StatefulPopover
      popoverMargin={8}
      content={<QuestionButtonPopoverContent />}
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
      />
    </StatefulPopover>
  );
};

const QuestionButtonPopoverContent = () => {
  const items: Items = [
    {
      label: "Documentation",
      endEnhancer: <ArrowUpRightIcon />,
      href: import.meta.env.VITE_SANDBOX_DOCS_URL,
    },
    {
      label: "Support",
      endEnhancer: <ArrowUpRightIcon />,
      disabled: true,
    },
    {
      label: "Share feedback",
      href: "",
      disabled: true,
    },
  ];

  const menuOverrides: MenuOverrides = {
    List: {
      style: {
        backgroundColor: COLORS.gray800,
      },
    },
  };

  return <Menu isDropdown items={items} size={MENU_SIZE.small} overrides={menuOverrides} />;
};
