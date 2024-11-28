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
import { Link, ShareIcon } from "../../shared";
import { addressRoute } from "../../routing";

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
      <Link to={addressRoute} params={{ address }} target="_blank">
        <StatefulTooltip content="Open in Explorer" showArrow={false} placement="top">
          <ButtonIcon icon={<ShareIcon />} kind={BUTTON_KIND.secondary} />
        </StatefulTooltip>
      </Link>
    </div>
  );
};
