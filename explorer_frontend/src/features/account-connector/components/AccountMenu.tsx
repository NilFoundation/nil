import { useStyletron } from "styletron-react";
import { styles } from "./styles";
import { BUTTON_KIND, Button, COLORS, CopyButton, LabelLarge, LabelMedium } from "@nilfoundation/ui-kit";
import { EndpointInput } from "./EndpointInput";
import {
  $balance,
  $balanceCurrency,
  createWalletFx,
  regenrateAccountEvent,
  topUpEvent,
  topUpWalletBalanceFx,
} from "../models/model";
import { useUnit } from "effector-react";
import { OverflowEllipsis, measure } from "../../shared";
import type { ButtonOverrides } from "baseui/button";
import { Token } from "./Token";
import { expandProperty } from "inline-style-expand-shorthand";

type AccountMenuProps = {
  address: string | null;
};

const btnOverrides: ButtonOverrides = {
  Root: {
    style: ({ $disabled }) => ({
      backgroundColor: $disabled ? `${COLORS.gray400}!important` : "",
    }),
  },
};

const AccountMenu = ({ address }: AccountMenuProps) => {
  const [css] = useStyletron();
  const [isPendingWalletCreation] = useUnit([createWalletFx.pending]);
  const [balance, balanceCurrency] = useUnit([$balance, $balanceCurrency]);
  const [isPendingTopUp] = useUnit([topUpWalletBalanceFx.pending]);
  const displayBalance = balance === null ? "-" : measure(balance);

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: "24px",
        paddingTop: "24px",
        paddingBottom: "24px",
      })}
    >
      <LabelLarge>Wallet</LabelLarge>
      <div>
        {address !== null && (
          <div
            className={css({
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: "1ch",
            })}
          >
            <LabelMedium width="250px" color={COLORS.gray200}>
              <OverflowEllipsis>{address}</OverflowEllipsis>
            </LabelMedium>
            <CopyButton textToCopy={address} disabled={address === null} color={COLORS.gray200} />
          </div>
        )}
        {isPendingWalletCreation && <LabelMedium>Creating new wallet...</LabelMedium>}
      </div>
      <ul>
        <li className={css(styles.divider)} />
        <li className={css(styles.menuItem)}>
          <EndpointInput />
        </li>
        <li className={css(styles.divider)} />
        <li className={css(styles.menuItem)}>
          <Token balance={displayBalance} name="ETH" isMain />
        </li>
        {balanceCurrency !== null &&
          Object.keys(balanceCurrency).length !== 0 &&
          Object.entries(balanceCurrency).map(([name, balance]) => (
            <>
              <li key={`divider-${name}`} className={css(styles.divider)} />
              <li key={name} className={css(styles.menuItem)}>
                <Token balance={`${balance} ${name}`} name={name} isMain={false} />
              </li>
            </>
          ))}
        <li className={css(styles.divider)} />
        <li className={css({ ...styles.menuItem, ...expandProperty("padding", "24px 24px 0 24px") })}>
          <Button
            kind={BUTTON_KIND.primary}
            onClick={() => topUpEvent()}
            isLoading={isPendingTopUp}
            disabled={isPendingTopUp || isPendingWalletCreation}
            overrides={btnOverrides}
            className={css({
              whiteSpace: "nowrap",
            })}
          >
            Top up wallet
          </Button>
          <Button
            kind={BUTTON_KIND.toggle}
            onClick={() => regenrateAccountEvent()}
            className={css({
              whiteSpace: "nowrap",
            })}
          >
            Regenerate account
          </Button>
        </li>
      </ul>
    </div>
  );
};

export { AccountMenu };
