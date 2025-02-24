import { Button, COLORS, HeadingMedium, Input } from "@nilfoundation/ui-kit";
import { useStore } from "effector-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { Box, InputErrorMessage, ScreenHeader } from "../../features/components/shared";
import { $balanceToken, addToken } from "../../features/store/model/token.ts";
import { WalletRoutes } from "../../router";

export const AddCustomToken = () => {
  const { t } = useTranslation("translation");
  const [tokenName, setTokenName] = useState("");
  const [tokenAddress, setTokenAddress] = useState("");
  const [addressError, setAddressError] = useState("");

  const navigate = useNavigate();
  const balanceCurrency = useStore($balanceToken);

  const handleTokenNameInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    setTokenName(e.target.value);
  };
  const handleTokenAddressInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    setTokenAddress(e.target.value);
  };

  const validateToken = (address: string) => {
    const isHex = /^0x[a-fA-F0-9]{40}$/.test(address);
    if (!isHex) {
      return false;
    }
    return !balanceCurrency || !balanceCurrency[address];
  };

  return (
    <Box
      $style={{
        display: "flex",
        flexDirection: "column",
        height: "100vh",
        padding: "24px",
        boxSizing: "border-box",
        overflowY: "auto",
        flex: 1,
      }}
    >
      <ScreenHeader
        route={WalletRoutes.WALLET.BASE}
        title={t("wallet.manageTokens.addTokenPage.title")}
      />
      <Box
        $style={{
          padding: "3px",
          "-ms-overflow-style": "none",
          margin: "24px 0px 12px 0px",
        }}
      >
        <HeadingMedium $style={{ color: COLORS.gray50, margin: "4px 0px" }}>
          {t("wallet.manageTokens.addTokenPage.nameLabel")}
        </HeadingMedium>
        <Input
          placeholder={t("wallet.manageTokens.addTokenPage.namePlaceholder")}
          name="tokenName"
          value={tokenName}
          onChange={handleTokenNameInput}
        />

        <Box $style={{ margin: "8px" }} />

        <HeadingMedium $style={{ color: COLORS.gray50, margin: "4px 0px" }}>
          {t("wallet.manageTokens.addTokenPage.addressLabel")}
        </HeadingMedium>
        <Input
          placeholder={t("wallet.manageTokens.addTokenPage.addressPlaceholder")}
          name="tokenAddress"
          value={tokenAddress}
          onChange={handleTokenAddressInput}
        />
        {addressError && <InputErrorMessage error={addressError} style={{ marginTop: "5px" }} />}
      </Box>

      <Box
        $style={{
          position: "absolute",
          bottom: "24px",
          zIndex: 100,
          width: "87%",
          marginRight: "24px",
        }}
      >
        <Button
          onClick={() => {
            if (!validateToken(tokenAddress)) {
              setAddressError(t("wallet.manageTokens.addTokenPage.addressError"));
              return;
            }
            addToken({ name: tokenName, address: tokenAddress });
            navigate(WalletRoutes.WALLET.BASE);
          }}
          overrides={{
            Root: {
              style: {
                width: "100%",
                height: "48px",
                backgroundColor: COLORS.gray800,
                color: COLORS.gray200,
                ":hover": {
                  backgroundColor: COLORS.gray700,
                },
              },
            },
          }}
        >
          {t("wallet.manageTokens.addTokenPage.button")}
        </Button>
      </Box>
    </Box>
  );
};
