import { BUTTON_KIND, ButtonIcon, COLORS, DownloadIcon, StatefulTooltip } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { exportApp } from "../models/exportApp.ts";
import { isTutorialPage } from "../../code/model.ts";
import { useUnit } from "effector-react";

type DownloadAppButtonProps = {
  disabled?: boolean;
  kind?: BUTTON_KIND;
};

export const DownloadAppButton: FC<DownloadAppButtonProps> = ({
  disabled,
  kind = BUTTON_KIND.text,
}) => {
  const isTutorial = useUnit(isTutorialPage);
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
              backgroundColor: isTutorial ? COLORS.blue800 : COLORS.gray800,
              ":hover": {
                backgroundColor: isTutorial ? COLORS.blue700 : COLORS.gray700,
              }
            },
          },
        }}
      />
    </StatefulTooltip>
  );
};
