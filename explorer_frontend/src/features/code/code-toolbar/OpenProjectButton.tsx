import {
  BUTTON_KIND,
  BUTTON_SIZE,
  COLORS,
  ChevronDownIcon,
  ChevronUpIcon,
  type Items,
  MENU_SIZE,
  Menu,
} from "@nilfoundation/ui-kit";
import { Button } from "baseui/button";
import type { MenuOverrides } from "baseui/menu";
import { useUnit } from "effector-react";
import { type FC, useState } from "react";
import { useStyletron } from "styletron-react";
import { StatefulPopover, useMobile } from "../../shared";
import AsyncCallExample from "../assets/AsyncCallExample.sol";
import AsyncRequestExample from "../assets/AsyncRequestExample.sol";
import HandlingExtTxExample from "../assets/HandlingExtTxExample.sol";
import TokenExample from "../assets/TokenExample.sol";
import { $recentProjects, changeCode, updateRecentProjects } from "../model";
import { compile } from "../model";

type OpenProjectButtonProps = {
  disabled?: boolean;
};

export const OpenProjectButton: FC<OpenProjectButtonProps> = ({ disabled }) => {
  const recentProjects = useUnit($recentProjects);
  const anyRecentProjects = Object.keys(recentProjects).length > 0;
  const [isOpen, setIsOpen] = useState(false);
  const [css] = useStyletron();
  const [isMobile] = useMobile();
  const btnOverrides = {
    Root: {
      style: {
        whiteSpace: "nowrap",
        ...(!isMobile
          ? {
              paddingLeft: "24px",
              paddingRight: "24px",
            }
          : {
              paddingLeft: "12px",
              paddingRight: "12px",
            }),
      },
    },
  };

  const menuItems = {
    "Tutorial examples:": [
      {
        label: "Async call",
        onChange: () => {
          updateRecentProjects();
          changeCode(AsyncCallExample);
          compile();
        },
      },
      {
        label: "Async request with response guarantee",
        onChange: () => {
          updateRecentProjects();
          changeCode(AsyncRequestExample);
          compile();
        },
      },
      {
        label: "Handling external transactions",
        onChange: () => {
          updateRecentProjects();
          changeCode(HandlingExtTxExample);
          compile();
        },
      },
      {
        label: "Tokens",
        onChange: () => {
          updateRecentProjects();
          changeCode(TokenExample);
          compile();
        },
      },
      ...(anyRecentProjects ? [{ divider: true }] : []),
    ],
    ...(anyRecentProjects
      ? {
          "Recent projects:": Object.entries(recentProjects).map(([label, code]) => ({
            label,
            onChange: () => {
              changeCode(code);
              compile();
            },
          })),
        }
      : {}),
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
            item?.onChange();
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
          height: isMobile ? "32px" : "46px",
          flexShrink: 0,
        })}
        disabled={disabled}
        overrides={btnOverrides}
        endEnhancer={isOpen ? <ChevronUpIcon /> : <ChevronDownIcon />}
        data-testid="examples-dropdown-button"
      >
        Open
      </Button>
    </StatefulPopover>
  );
};
