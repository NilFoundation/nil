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
import { transactionRoute } from "../../routing";

type TransactionSentLogProps = {
  hash: string;
};

export const TransactionSentLog: FC<TransactionSentLogProps> = ({ hash }) => {
  const [css] = useStyletron();

  return (
    <div
      className={css({
        display: "flex",
        gap: "8px",
        alignItems: "center",
      })}
    >
      <LabelMedium color={COLORS.gray400}>Transaction hash:</LabelMedium>
      <LabelMedium color={COLORS.gray50}>{hash}</LabelMedium>
      <CopyButton kind={BUTTON_KIND.secondary} textToCopy={hash} size={BUTTON_SIZE.default} />
      <Link to={transactionRoute} params={{ hash }} target="_blank">
        <StatefulTooltip content="Open in Explorer" showArrow={false} placement="top">
          <ButtonIcon icon={<ShareIcon />} kind={BUTTON_KIND.secondary} />
        </StatefulTooltip>
      </Link>
    </div>
  );
};
