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
import { useStyletron } from "baseui";
import { Button } from "baseui/button";
import type { MenuOverrides } from "baseui/menu";
import { type FC, useState } from "react";
import { StatefulPopover } from "../../shared";
import { $availableSolidityVersions, changeSolidityVersion } from "../model";

type CompilerVersionButtonProps = {
  disabled?: boolean;
  isMobile?: boolean;
};

export const CompilerVersionButton: FC<CompilerVersionButtonProps> = ({ disabled, isMobile }) => {
  const [isOpen, setIsOpen] = useState(false);
  const [css, theme] = useStyletron();
  const height = isMobile ? "48px" : "46px";
  const btnOverrides = {
    Root: {
      style: {
        whiteSpace: "nowrap",
        borderRadius: "0 8px 8px 0",
        backgroundColor: theme.colors.primaryButtonBackgroundColor,
        ":hover": {
          backgroundColor: theme.colors.primaryButtonBackgroundHoverColor,
        },
        paddingLeft: "0px !important",
        paddingRight: "12px !important",
        height: height,
      },
    },
  };

  const versions = $availableSolidityVersions.getState().map((v) => {
    return { label: v };
  });

  const menuOverrides: MenuOverrides = {
    List: {
      style: {
        backgroundColor: COLORS.gray800,
        width: "110%",
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
            changeSolidityVersion(item.label);
            close();
          }}
          items={versions as Items}
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
        kind={BUTTON_KIND.primary}
        size={isMobile ? BUTTON_SIZE.compact : BUTTON_SIZE.large}
        className={css({
          height: isMobile ? "32px" : "46px",
          flexShrink: 0,
        })}
        overrides={btnOverrides}
        endEnhancer={isOpen ? <ChevronUpIcon /> : <ChevronDownIcon />}
        disabled={disabled}
      />
    </StatefulPopover>
  );
};
