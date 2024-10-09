import { useStyletron } from "styletron-react";
import {
  CaretUpIcon,
  LabelMedium,
  CaretDownIcon,
  COLORS,
  ButtonIcon,
  MenuIcon,
  BUTTON_KIND,
} from "@nilfoundation/ui-kit";
import { useAccountConnector } from "..";
import { memo, useState } from "react";
import { Popover } from "./Popover";
import { styles } from "./styles";
import { AccountMenu } from "./AccountMenu";
import { OverflowEllipsis, useMobile } from "../../shared";

const MemoizedAccountMenu = memo(AccountMenu);

const AccountContent = () => {
  const [isMobile] = useMobile();
  const [css] = useStyletron();
  const { wallet } = useAccountConnector();
  const address = wallet ? wallet.getAddressHex() : null;
  const text = address ? address : "Not connected";
  const isAccountConnected = !!wallet;

  const [isOpen, setIsOpen] = useState(false);
  const Icon = isOpen ? CaretUpIcon : CaretDownIcon;

  return (
    <Popover
      onOpen={() => setIsOpen(true)}
      onClose={() => setIsOpen(false)}
      popoverMargin={20}
      content={<MemoizedAccountMenu address={address} />}
    >
      {isMobile ? (
        <ButtonIcon kind={BUTTON_KIND.secondary} icon={<MenuIcon />} />
      ) : (
        <button className={css(styles.account)} type="button">
          <div className={css({ ...styles.indicator, ...(isAccountConnected ? styles.activeIndicator : {}) })} />
          <LabelMedium className={css(styles.label)}>
            <OverflowEllipsis>{text}</OverflowEllipsis>
          </LabelMedium>
          <Icon className={css(styles.icon)} color={COLORS.gray200} />
        </button>
      )}
    </Popover>
  );
};

export { AccountContent };
