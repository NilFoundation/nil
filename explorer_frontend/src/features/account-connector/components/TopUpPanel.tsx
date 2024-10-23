import { Button, COLORS, Input } from "@nilfoundation/ui-kit";
import { LabelMedium, LabelSmall } from "baseui/typography";
import { useStyletron } from "styletron-react";
import { BackLink } from "./BackLink";
import {
  $topupInput,
  $wallet,
  setActiveComponent,
  setTopupInput,
  topupCurrencyEvent,
  topupWalletCurrencyFx,
} from "../models/model";
import { ActiveComponent } from "../ActiveComponent";
import { FormControl } from "baseui/form-control";
import { useUnit } from "effector-react";
import { CurrencyInput } from "../../currencies";
import type { InputOverrides } from "baseui/input";
import { $faucets } from "../../currencies/model";

const inputOverrides: InputOverrides = {
  Root: {
    style: {
      backgroundColor: COLORS.gray700,
      ":hover": {
        backgroundColor: COLORS.gray600,
      },
    },
  },
};

const TopUpPanel = () => {
  const [css] = useStyletron();
  const [wallet, faucets, topupInput, topupInProgress, successTopup] = useUnit([
    $wallet,
    $faucets,
    $topupInput,
    topupWalletCurrencyFx.pending,
    topupWalletCurrencyFx.done,
  ]);
  const availiableTokens = Object.keys(faucets ?? {});

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        paddingRight: "24px",
      })}
    >
      <BackLink
        title="Back"
        goBackCb={() => setActiveComponent(ActiveComponent.Main)}
        disabled={topupInProgress}
      />
      <div
        className={css({
          width: "100%",
          marginTop: "8px",
        })}
      >
        <FormControl label={<LabelMedium>To</LabelMedium>}>
          <Input readOnly placeholder={wallet?.getAddressHex() ?? ""} overrides={inputOverrides} />
        </FormControl>
      </div>
      <div
        className={css({
          width: "100%",
        })}
      >
        <CurrencyInput
          label="Amount"
          currencies={availiableTokens.map((t) => ({
            currency: t,
          }))}
          onChange={({ amount, currency }) => {
            setTopupInput({
              currency,
              amount,
            });
          }}
          value={topupInput}
        />
      </div>
      <Button
        className={css({
          width: "100%",
          marginTop: "8px",
          marginBottom: "16px",
        })}
        onClick={() => topupCurrencyEvent()}
        disabled={topupInProgress || topupInput.amount === ""}
        isLoading={topupInProgress}
        overrides={{
          Root: {
            style: ({ $disabled }) => ({
              height: "48px",
              backgroundColor: $disabled ? `${COLORS.gray400}!important` : "",
            }),
          },
        }}
      >
        Top up
      </Button>
      <LabelSmall
        color={COLORS.gray200}
        className={css({
          textAlign: "center",
          display: "inline-block",
          fontSize: "14px",
        })}
      >
        <a
          href={import.meta.env.VITE_SANDBOX_MULTICURRENCY_URL}
          target="_blank"
          rel="noreferrer"
          className={css({
            textDecoration: "underline",
          })}
        >
          Learn
        </a>{" "}
        how to handle tokens and multi-currencies in your environment.
      </LabelSmall>
    </div>
  );
};

export { TopUpPanel };
