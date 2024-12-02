import {
  BUTTON_KIND,
  BUTTON_SIZE,
  type Items,
  MENU_SIZE,
  Menu,
  COLORS,
  ChevronDownIcon,
  ChevronUpIcon,
} from "@nilfoundation/ui-kit";
import { StatefulPopover, useMobile } from "../../shared";
import type { MenuOverrides } from "baseui/menu";
import { Button } from "baseui/button";
import { useState, type FC } from "react";

type ExamplesButtonProps = {
  disabled?: boolean;
};

export const ExamplesButton: FC<ExamplesButtonProps> = ({ disabled }) => {
  const [isOpen, setIsOpen] = useState(false);
  const [isMobile] = useMobile();
  const btnOverrides = {
    Root: {
      style: {
        whiteSpace: "nowrap",
        ...(!isMobile ? { paddingLeft: "24px", paddingRight: "24px" } : {}),
      },
    },
  };

  return (
    <StatefulPopover
      onOpen={() => setIsOpen(true)}
      onClose={() => setIsOpen(false)}
      popoverMargin={8}
      content={<ExamplesButtonPopoverContent />}
      placement={isMobile ? "bottomRight" : "bottomLeft"}
      autoFocus
      triggerType="click"
    >
      <Button
        kind={BUTTON_KIND.secondary}
        size={isMobile ? BUTTON_SIZE.compact : BUTTON_SIZE.large}
        disabled={disabled}
        overrides={btnOverrides}
        endEnhancer={isOpen ? <ChevronUpIcon /> : <ChevronDownIcon />}
      >
        Examples
      </Button>
    </StatefulPopover>
  );
};

const ExamplesButtonPopoverContent = () => {
  const items = {
    "Tutorial examples:": [{ label: "Async", disabled: true }],
  };

  const menuOverrides: MenuOverrides = {
    List: {
      style: {
        backgroundColor: COLORS.gray800,
      },
    },
  };

  return (
    <Menu items={items as unknown as Items} size={MENU_SIZE.small} overrides={menuOverrides} />
  );
};
