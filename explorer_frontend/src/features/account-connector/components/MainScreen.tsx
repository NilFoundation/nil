import { useStyletron } from "styletron-react";
import { styles } from "./styles";
import {
  BUTTON_KIND,
  Button,
  COLORS,
  CopyButton,
  LabelLarge,
  LabelMedium,
  LabelSmall,
  PlusIcon,
} from "@nilfoundation/ui-kit";
import { EndpointInput } from "./EndpointInput";
import {
  $balance,
  $balanceCurrency,
  $initializingWalletError,
  $initializingWalletState,
  $wallet,
  createWalletFx,
  regenrateAccountEvent,
  setActiveComponent,
  topUpWalletBalanceFx,
} from "../models/model";
import { useUnit } from "effector-react";
import { OverflowEllipsis } from "../../shared";
import type { ButtonOverrides } from "baseui/button";
import { Token } from "./Token";
import { ActiveComponent } from "../ActiveComponent";
import { formatEther } from "viem";

const btnOverrides: ButtonOverrides = {
  Root: {
    style: ({ $disabled }) => ({
      backgroundColor: $disabled ? `${COLORS.gray400}!important` : "",
      width: "50%",
    }),
  },
};

const MainScreen = () => {
  const [css] = useStyletron();
  const [isPendingWalletCreation] = useUnit([createWalletFx.pending]);
  const [wallet, balance, balanceCurrency, initializingWalletState, initializingWalletError] =
    useUnit([
      $wallet,
      $balance,
      $balanceCurrency,
      $initializingWalletState,
      $initializingWalletError,
    ]);
  const [isPendingTopUp] = useUnit([topUpWalletBalanceFx.pending]);
  const displayBalance = balance === null ? "-" : formatEther(balance);
  const address = wallet ? wallet.getAddressHex() : null;

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: "24px",
        maxHeight: "531px",
        overflowY: "auto",
        overscrollBehavior: "contain",
        paddingRight: "24px",
      })}
    >
      <div
        className={css({
          width: "100%",
          display: "flex",
          flexDirection: "column",
          position: "sticky",
          alignItems: "center",
          gap: "24px",
          top: 0,
          backgroundColor: COLORS.gray800,
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
          {(isPendingWalletCreation || initializingWalletError) && (
            <div>
              <LabelMedium
                className={css({
                  textAlign: "center",
                })}
              >
                Creating new wallet
              </LabelMedium>
              <LabelSmall
                color={initializingWalletError ? COLORS.red200 : COLORS.gray400}
                className={css({
                  textAlign: "center",
                })}
              >
                {initializingWalletError ? initializingWalletError : initializingWalletState}
              </LabelSmall>
            </div>
          )}
        </div>
        <EndpointInput disabled={isPendingWalletCreation} />
      </div>
      <ul
        className={css({
          width: "100%",
        })}
      >
        <li className={css(styles.menuItem)}>
          <Token balance={displayBalance} name="ETH" isMain />
        </li>
        {balanceCurrency !== null &&
          Object.keys(balanceCurrency).length !== 0 &&
          Object.entries(balanceCurrency).map(([name, balance]) => (
            <>
              <li key={`divider-${name}`} className={css(styles.divider)} />
              <li key={name} className={css(styles.menuItem)}>
                <Token balance={balance.toString()} name={name} isMain={false} />
              </li>
            </>
          ))}
        <li
          className={css({
            ...styles.menuItem,
            paddingTop: "24px",
            paddingLeft: 0,
            paddingRight: 0,
            position: "sticky",
            bottom: 0,
            backgroundColor: COLORS.gray800,
          })}
        >
          <Button
            kind={BUTTON_KIND.primary}
            onClick={() => setActiveComponent(ActiveComponent.Topup)}
            isLoading={isPendingTopUp}
            disabled={isPendingTopUp || isPendingWalletCreation || !wallet}
            overrides={btnOverrides}
            className={css({
              whiteSpace: "nowrap",
            })}
          >
            <PlusIcon size={24} />
            Top up
          </Button>
          <Button
            kind={BUTTON_KIND.toggle}
            onClick={() => regenrateAccountEvent()}
            className={css({
              whiteSpace: "nowrap",
            })}
            disabled={isPendingTopUp || isPendingWalletCreation}
            overrides={{
              Root: {
                style: {
                  width: "50%",
                },
              },
            }}
          >
            Regenerate account
          </Button>
        </li>
      </ul>
    </div>
  );
};

export { MainScreen };
