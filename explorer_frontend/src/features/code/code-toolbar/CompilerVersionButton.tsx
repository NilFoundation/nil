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
import { $availableSolidityVersions, $solidityVersion, changeSolidityVersion } from "../model";
import { useStyletron } from "styletron-react";


export const CompilerVersionButton: FC<any> = () => {
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

  const versions = $availableSolidityVersions.getState().map((v) => {
    return { label: v }; // Explicitly return an object with the label property
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
        kind={BUTTON_KIND.secondary}
        size={isMobile ? BUTTON_SIZE.compact : BUTTON_SIZE.large}
        className={css({
          height: isMobile ? "32px" : "48px",
          flexShrink: 0,
        })}
        overrides={btnOverrides}
        endEnhancer={isOpen ? <ChevronUpIcon /> : <ChevronDownIcon />}
      >
        Compiler {$solidityVersion.getState().split("+")[0]}
      </Button>
    </StatefulPopover>
  );
};
