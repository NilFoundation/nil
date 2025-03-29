import {
  BUTTON_KIND,
  Button,
  ButtonIcon,
  COLORS,
  LabelMedium,
  MenuIcon,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { ChevronDown, ChevronUp } from "baseui/icon";
import { useUnit } from "effector-react";
import { memo, useState } from "react";
import { OverflowEllipsis, StatefulPopover, useMobile } from "../../shared";
import { $smartAccount } from "../model";
import { AccountContainer } from "./AccountContainer";
import { styles } from "./styles";

const MemoizedAccountContainer = memo(AccountContainer);

const AccountContent = () => {
  const [isMobile] = useMobile();
  const [css, theme] = useStyletron();
  const [smartAccount] = useUnit([$smartAccount]);
  const address = smartAccount ? smartAccount.address : null;
  const text = address ? address : "Not connected";
  const isAccountConnected = !!smartAccount;
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
      overrides={{
        Inner: {
          style: {
            backgroundColor: theme.colors.backgroundSecondary,
          },
        },
      }}
    >
      {isMobile ? (
        <ButtonIcon
          kind={BUTTON_KIND.secondary}
          icon={<MenuIcon />}
          overrides={{
            Root: {
              style: {
                backgroundColor: theme.colors.inputButtonAndDropdownOverrideBackgroundColor,
                ":hover": {
                  backgroundColor: theme.colors.inputButtonAndDropdownOverrideBackgroundHoverColor,
                },
              },
            },
          }}
        />
      ) : (
        <Button
          kind={BUTTON_KIND.secondary}
          className={css(styles.account)}
          type="button"
          overrides={{
            Root: {
              style: {
                backgroundColor: theme.colors.inputButtonAndDropdownOverrideBackgroundColor,
                ":hover": {
                  backgroundColor: theme.colors.inputButtonAndDropdownOverrideBackgroundHoverColor,
                },
              },
            },
          }}
        >
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
