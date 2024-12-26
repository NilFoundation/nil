import { BUTTON_KIND, ButtonIcon, StatefulTooltip } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { DeleteIcon } from "../DeleteIcon.tsx";
import { removeValueInput } from "../../models/base.ts";

type RemoveTokenButtonProps = {
  index: number;
  kind?: BUTTON_KIND;
};

export const RemoveTokenButton: FC<RemoveTokenButtonProps> = ({
  index,
  kind = BUTTON_KIND.text,
}) => {
  return (
    <StatefulTooltip content="Remove token" showArrow={false} placement="bottom" popoverMargin={6}>
      <ButtonIcon
        disabled={false}
        icon={<DeleteIcon />}
        kind={kind}
        onClick={(e) => {
          e.stopPropagation();
          removeValueInput(index);
        }}
        overrides={{
          Root: {
            style: {
              marginBottom: "16px",
              width: "46px",
              height: "46px",
            },
          },
        }}
      />
    </StatefulTooltip>
  );
};
