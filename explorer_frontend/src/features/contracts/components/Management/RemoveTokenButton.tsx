import { BUTTON_KIND, ButtonIcon, StatefulTooltip } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { removeValueInput } from "../../models/base.ts";
import { DeleteIcon } from "../DeleteIcon.tsx";

type RemoveTokenButtonProps = {
  index: number;
  kind?: BUTTON_KIND;
  functionName: string;
};

export const RemoveTokenButton: FC<RemoveTokenButtonProps> = ({
  index,
  kind = BUTTON_KIND.text,
  functionName,
}) => {
  return (
    <StatefulTooltip content="Remove token" showArrow={false} placement="bottom" popoverMargin={6}>
      <ButtonIcon
        disabled={false}
        icon={<DeleteIcon />}
        kind={kind}
        onClick={(e) => {
          e.stopPropagation();
          removeValueInput({ functionName, index });
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
