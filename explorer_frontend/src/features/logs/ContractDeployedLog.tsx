import type { FC } from "react";
import {
  BUTTON_KIND,
  BUTTON_SIZE,
  ButtonIcon,
  COLORS,
  CopyButton,
  LabelMedium,
  StatefulTooltip,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import { ShareIcon } from "./ShareIcon";
import { Link } from "../shared";
import { addressRoute } from "../routing";

type ContractDeployedLogProps = {
  address: string;
};

export const ContractDeployedLog: FC<ContractDeployedLogProps> = ({ address }) => {
  const [css] = useStyletron();

  return (
    <div
      className={css({
        display: "flex",
        gap: "8px",
        alignItems: "center",
      })}
    >
      <LabelMedium color={COLORS.gray400}>Contract address:</LabelMedium>
      <LabelMedium color={COLORS.gray50}>{address}</LabelMedium>
      <CopyButton kind={BUTTON_KIND.secondary} textToCopy={address} size={BUTTON_SIZE.default} />
      <StatefulTooltip content="Open in Explorer" showArrow placement="bottom">
        <Link to={addressRoute} params={{ address }}>
          <ButtonIcon icon={<ShareIcon />} kind={BUTTON_KIND.secondary} />
        </Link>
      </StatefulTooltip>
    </div>
  );
};
