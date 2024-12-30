import { BUTTON_KIND, ButtonIcon, DownloadIcon, StatefulTooltip } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { exportApp } from "../models/exportApp.ts";

type DownloadAppButtonProps = {
  disabled?: boolean;
  kind?: BUTTON_KIND;
};

export const DownloadAppButton: FC<DownloadAppButtonProps> = ({
  disabled,
  kind = BUTTON_KIND.text,
}) => {
  return (
    <StatefulTooltip
      content="Download contract and compilation artifacts"
      showArrow={false}
      placement="bottom"
      popoverMargin={0}
    >
      <ButtonIcon
        disabled={disabled}
        icon={<DownloadIcon />}
        kind={kind}
        onClick={() => exportApp()}
        overrides={{
          Root: {
            style: {
              paddingTop: "6px",
              paddingBottom: "6px",
              paddingLeft: "6px",
              paddingRight: "6px",
              width: "32px",
              height: "32px",
            },
          },
        }}
      />
    </StatefulTooltip>
  );
};
