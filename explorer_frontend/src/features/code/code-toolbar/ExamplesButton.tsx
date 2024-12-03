import {
  BUTTON_KIND,
  BUTTON_SIZE,
  Menu,
  COLORS,
  ChevronDownIcon,
  ChevronUpIcon,
  MENU_SIZE,
  type Items,
} from "@nilfoundation/ui-kit";
import { StatefulPopover, useMobile } from "../../shared";
import type { MenuOverrides } from "baseui/menu";
import { Button } from "baseui/button";
import { useState, type FC } from "react";
import { changeCode } from "../model";
import { useStyletron } from "styletron-react";
import AsyncCallExample from "../assets/AsyncCallExample.sol";

type ExamplesButtonProps = {
  disabled?: boolean;
};

export const ExamplesButton: FC<ExamplesButtonProps> = ({ disabled }) => {
  const [isOpen, setIsOpen] = useState(false);
  const [css] = useStyletron();
  const [isMobile] = useMobile();
  const btnOverrides = {
    Root: {
      style: {
        whiteSpace: "nowrap",
        ...(!isMobile ? { paddingLeft: "24px", paddingRight: "24px" } : {}),
      },
    },
  };

  const menuItems = {
    "Tutorial examples:": [{ label: "Async call", exampleCode: AsyncCallExample }],
  };
  const menuOverrides: MenuOverrides = {
    List: {
      style: {
        backgroundColor: COLORS.gray800,
      },
    },
  };

  return (
    <StatefulPopover
      onOpen={() => setIsOpen(true)}
      onClose={() => setIsOpen(false)}
      popoverMargin={8}
      content={({ close }) => (
        <Menu
          onItemSelect={({ item }) => {
            changeCode(item.exampleCode);
            close();
          }}
          items={menuItems as unknown as Items}
          size={MENU_SIZE.small}
          overrides={menuOverrides}
          renderAll
          isDropdown
        />
      )}
      placement={isMobile ? "bottomRight" : "bottomLeft"}
      autoFocus
      triggerType="click"
    >
      <Button
        kind={BUTTON_KIND.secondary}
        size={isMobile ? BUTTON_SIZE.compact : BUTTON_SIZE.large}
        className={css({
          height: isMobile ? "32px" : "48px",
          flexShrink: 0,
        })}
        disabled={disabled}
        overrides={btnOverrides}
        endEnhancer={isOpen ? <ChevronUpIcon /> : <ChevronDownIcon />}
      >
        Examples
      </Button>
    </StatefulPopover>
  );
};
