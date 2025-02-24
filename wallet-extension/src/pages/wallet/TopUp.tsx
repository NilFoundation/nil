import type { FaucetClient, SmartAccountV1 } from "@nilfoundation/niljs";
import {
  Button,
  COLORS,
  HeadingMedium,
  NOTIFICATION_KIND,
  Notification,
  ParagraphXSmall,
} from "@nilfoundation/ui-kit";
import { useStore } from "effector-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { topUpSpecificCurrency } from "../../features/blockchain";
import { Currency } from "../../features/components/currency";
import {
  Box,
  CurrencyInput,
  InputErrorMessage,
  MainAddressInput,
  ScreenHeader,
} from "../../features/components/shared";
import { $faucetClient } from "../../features/store/model/blockchain";
import { $smartAccount } from "../../features/store/model/smartAccount.ts";
import { $tokens } from "../../features/store/model/token.ts";
import { convertTopUpAmount, getQuickAmounts, validateTopUpAmount } from "../../features/utils";
import { getTopupCurrencies } from "../../features/utils/token.ts";
import { WalletRoutes } from "../../router";

export const TopUp = () => {
  const { t } = useTranslation("translation");
  const navigate = useNavigate();
  const [inputError, setInputError] = useState("");
  const [topUpError, setTopUpError] = useState("");
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const smartAccount = useStore($smartAccount);
  const faucetClient = useStore($faucetClient);
  const tokens = useStore($tokens);

  const currencies = getTopupCurrencies(tokens);
  const [selectedCurrency, setSelectedCurrency] = useState(currencies[0]);
  const [amount, setAmount] = useState("");

  const quickAmounts = getQuickAmounts(selectedCurrency.label);

  const handleCurrencyChange = (params: { value: { label: string }[] }) => {
    const selected = currencies.find((currency) => currency.label === params.value[0].label);
    if (selected) {
      setInputError("");
      setSelectedCurrency(selected);
      setTopUpError("");
    }
  };

  const handleAmountChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setInputError("");
    setAmount(e.target.value);
  };

  const handleQuickAmountClick = (quickAmount: number) => {
    setInputError("");
    setAmount(quickAmount.toString());
  };

  const handleTopUp = async () => {
    setTopUpError("");

    if (!smartAccount) return console.error("SmartAccount is not initialized");

    const validationError = validateTopUpAmount(amount, selectedCurrency.label);
    if (validationError) {
      setInputError(validationError);
      return;
    }

    setIsLoading(true);
    try {
      if (!faucetClient) {
        console.error("FaucetClient null");
        return;
      }

      const finalAmount = convertTopUpAmount(amount, selectedCurrency.label);
      await topUpSpecificCurrency(
        smartAccount as SmartAccountV1,
        faucetClient as FaucetClient,
        selectedCurrency.label,
        finalAmount,
      );

      navigate(WalletRoutes.WALLET.BASE);
    } catch (error) {
      console.error("Top-up failed:", error);
      setTopUpError("Something went wrong, please try again");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box
      $align="stretch"
      $justify="flex-start"
      $padding="24px"
      $style={{ height: "100vh", boxSizing: "border-box" }}
    >
      <ScreenHeader route={WalletRoutes.WALLET.BASE} title={t("wallet.topUpPage.title")} />

      {/* To Section */}
      <Box $padding="24px 0">
        <HeadingMedium $style={{ color: COLORS.gray50, marginBottom: "12px" }}>
          {t("wallet.topUpPage.toSection")}
        </HeadingMedium>
        <MainAddressInput />
      </Box>

      {/* Amount Section */}
      <Box>
        <HeadingMedium $style={{ color: COLORS.gray50, marginBottom: "12px" }}>
          {t("wallet.topUpPage.amountSection")}
        </HeadingMedium>
        <CurrencyInput
          error={inputError}
          selectedCurrency={selectedCurrency}
          currencies={currencies}
          onCurrencyChange={handleCurrencyChange}
          value={amount}
          onChange={handleAmountChange}
        />

        {inputError && <InputErrorMessage error={inputError} style={{ marginTop: "5px" }} />}

        {/* Quick Amount Buttons */}
        <Box $gap="8px" $style={{ marginTop: "16px", flexDirection: "row" }}>
          {quickAmounts.map((quickAmount) => (
            <Box
              key={quickAmount}
              $align="center"
              $justify="center"
              onClick={() => handleQuickAmountClick(quickAmount)}
              $style={{
                width: "54px",
                height: "32px",
                backgroundColor: COLORS.gray800,
                color: COLORS.gray200,
                borderRadius: "8px",
                transition: "all 0.2s ease",
                cursor: "pointer",
                ":hover": { backgroundColor: COLORS.gray700 },
                ":active": { backgroundColor: COLORS.gray600, transform: "scale(0.98)" },
              }}
            >
              <ParagraphXSmall>{quickAmount}</ParagraphXSmall>
            </Box>
          ))}
        </Box>
      </Box>

      {/* Top Up Button */}
      <Box $align="center" $justify="center" $style={{ marginTop: "auto", width: "100%" }}>
        {/* Show NIL Faucet Warning */}
        {selectedCurrency.label === Currency.NIL && topUpError === "" && (
          <Notification
            closeable={true}
            kind={NOTIFICATION_KIND.warning}
            hideIcon={true}
            overrides={{
              Body: {
                style: {
                  backgroundColor: COLORS.yellow300,
                  marginLeft: 0,
                  marginRight: 0,
                  width: "100%",
                },
              },
            }}
          >
            The NIL faucet is capped. The amount received may be different than requested
          </Notification>
        )}

        {topUpError && (
          <Notification
            closeable
            kind={NOTIFICATION_KIND.negative}
            hideIcon
            overrides={{
              Body: {
                style: {
                  backgroundColor: COLORS.red300,
                  marginLeft: 0,
                  marginRight: 0,
                  width: "100%",
                },
              },
            }}
          >
            {topUpError}
          </Notification>
        )}

        <Button
          onClick={handleTopUp}
          isLoading={isLoading}
          overrides={{ Root: { style: { width: "100%", height: "48px" } } }}
        >
          {t("wallet.topUpPage.button")}
        </Button>
      </Box>
    </Box>
  );
};
