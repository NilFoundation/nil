import { useStyletron } from "styletron-react";
import {
  LabelMedium,
  COLORS,
  ButtonIcon,
  MenuIcon,
  BUTTON_KIND,
  Button,
} from "@nilfoundation/ui-kit";
import { memo, useState } from "react";
import { styles } from "./styles";
import { AccountContainer } from "./AccountContainer";
import { OverflowEllipsis, useMobile, StatefulPopover } from "../../shared";
import { useUnit } from "effector-react";
import { $wallet } from "../models/model";
import { ChevronDown, ChevronUp } from "baseui/icon";

const MemoizedAccountContainer = memo(AccountContainer);

const AccountContent = () => {
  const [isMobile] = useMobile();
  const [css] = useStyletron();
  const wallet = useUnit($wallet);
  const address = wallet ? wallet.address : null;
  const text = address ? address : "Not connected";
  const isAccountConnected = !!wallet;
  const [isOpen, setIsOpen] = useState(false);
  const Icon = isOpen ? ChevronUp : ChevronDown;

  return (
    <StatefulPopover
      onOpen={() => setIsOpen(true)}
      onClose={() => setIsOpen(false)}
      popoverMargin={16}
      content={<MemoizedAccountContainer />}
      placement="bottomRight"
      autoFocus
      triggerType="click"
    >
      {isMobile ? (
        <ButtonIcon kind={BUTTON_KIND.secondary} icon={<MenuIcon />} />
      ) : (
        <Button kind={BUTTON_KIND.secondary} className={css(styles.account)} type="button">
          <div
            className={css({
              ...styles.indicator,
              ...(isAccountConnected ? styles.activeIndicator : {}),
            })}
          />
          <LabelMedium className={css(styles.label)}>
            <OverflowEllipsis>{text}</OverflowEllipsis>
          </LabelMedium>
          <Icon size={24} className={css(styles.icon)} color={COLORS.gray200} />
        </Button>
      )}
    </StatefulPopover>
  );
};

export { AccountContent };
