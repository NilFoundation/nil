import type { Hex } from "@nilfoundation/niljs";
import { BUTTON_KIND, ButtonIcon, StatefulTooltip } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { FC } from "react";
import { unlinkApp } from "../models/base";
import { DeleteIcon } from "./DeleteIcon";

type RemoveAppButtonProps = {
  bytecode: Hex;
  address: Hex;
  disabled?: boolean;
  kind?: BUTTON_KIND;
};

export const RemoveAppButton: FC<RemoveAppButtonProps> = ({
  address,
  bytecode,
  disabled,
  kind = BUTTON_KIND.text,
}) => {
  const [css, theme] = useStyletron();
  return (
    <StatefulTooltip content="Remove app" showArrow={false} placement="bottom" popoverMargin={0}>
      <ButtonIcon
        disabled={disabled}
        icon={<DeleteIcon />}
        kind={kind}
        onClick={(e) => {
          e.stopPropagation();
          if (address)
            unlinkApp({
              app: bytecode,
              address: address,
            });
        }}
        onKeyDown={(e) => {
          e.stopPropagation();
          if (!(e.key === "Enter" || e.key === " ")) {
            return;
          }

          if (address)
            unlinkApp({
              app: bytecode,
              address: address,
            });
        }}
        overrides={{
          Root: {
            style: {
              paddingTop: "6px",
              paddingBottom: "6px",
              paddingLeft: "6px",
              paddingRight: "6px",
              width: "32px",
              height: "32px",
              backgroundColor: theme.colors.contractHeaderButtonBackgroundColor,
              ":hover": {
                backgroundColor: theme.colors.contractHeaderButtonBackgroundHoverColor,
              },
            },
          },
        }}
      />
    </StatefulTooltip>
  );
};
