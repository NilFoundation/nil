import type { Hex } from "@nilfoundation/niljs";
import {
  Button,
  COLORS,
  HeadingMedium,
  Input,
  ParagraphSmall,
  SearchIcon,
} from "@nilfoundation/ui-kit";
import { useStore, useUnit } from "effector-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import addOne from "../../../public/icons/add-one.svg";
import reduceOne from "../../../public/icons/reduce-one.svg";
import nilIcon from "../../../public/icons/token/nil.svg";
import { Box, Icon, ScreenHeader } from "../../features/components/shared";
import {
  $balance,
  $balanceToken,
  $tokens,
  getTokenSymbolByAddress,
  hideToken,
  showToken,
} from "../../features/store/model/token.ts";
import { convertWeiToEth, formatAddress, getTokenIcon } from "../../features/utils";
import { WalletRoutes } from "../../router";

export const ManageTokens = () => {
  const { t } = useTranslation("translation");
  const [searchValue, setSearchValue] = useState("");
  const navigate = useNavigate();
  const balance = useStore($balance);
  const [balanceToken, storedTokens] = useUnit([$balanceToken, $tokens]);

  const addCustomTokenNavigate = () => {
    navigate(WalletRoutes.WALLET.ADD_CUSTOM_TOKEN);
  };
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchValue(e.target.value);
  };

  const tokens = storedTokens
    .map((token) => {
      const title = getTokenSymbolByAddress(token.address);
      const tokenBalance = balanceToken?.[token.address] ?? 0n;
      if (token.address === "") {
        return {
          icon: nilIcon,
          title: "Nil",
          subtitle: "Native token",
          subtitleColor: COLORS.green200,
          rightText: balance !== null ? `${convertWeiToEth(balance)} NIL` : "0 NIL",
          rightTextColor: COLORS.gray50,
          address: "",
          show: token.show,
        };
      }
      return {
        icon: getTokenIcon(title),
        title: title,
        subtitle: title !== "" ? "Mock token" : formatAddress(token.address as Hex),
        subtitleColor: "gray",
        rightText: `${tokenBalance.toString()} ${title}`,
        rightTextColor: COLORS.gray50,
        address: token.address,
        show: token.show,
      };
    })
    .filter(
      (token) =>
        searchValue === "" || token.title.toLowerCase().includes(searchValue.toLowerCase()),
    );

  const activeTokens = tokens.filter((token) => token.show);
  const hiddenTokens = tokens.filter((token) => !token.show);

  return (
    <Box
      $style={{
        display: "flex",
        flexDirection: "column",
        height: "100vh",
        padding: "24px",
        boxSizing: "border-box",
        overflowY: "auto",
        overflowX: "hidden",
        flex: 1,
      }}
    >
      <ScreenHeader
        route={WalletRoutes.WALLET.BASE}
        title={t("wallet.manageTokens.manageTokensPage.title")}
      />
      <Box
        $style={{
          padding: "3px",
          "-ms-overflow-style": "none",
          margin: "24px 0px 12px 0px",
        }}
      >
        <Input
          startEnhancer={<SearchIcon />}
          placeholder={t("wallet.manageTokens.manageTokensPage.search")}
          name="tokenSearch"
          value={searchValue}
          onChange={handleInputChange}
        />
      </Box>

      {activeTokens.map((token, index) => (
        <Box
          key={`${token.title}-${index}`}
          $align="center"
          $justify="space-between"
          $style={{
            flexDirection: "row",
            width: "100%",
            padding: "5px",
          }}
        >
          <Box
            $align="center"
            $gap="8px"
            $style={{
              flexDirection: "row",
            }}
          >
            <Icon
              src={token.icon}
              alt={`${token.title} Icon`}
              size={64}
              iconSize="44"
              background="transparent"
            />
            <Box $align="flex-start" $style={{ flexDirection: "column", maxWidth: "200px" }}>
              <HeadingMedium
                $style={{
                  color: COLORS.gray50,
                  whiteSpace: "nowrap",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                }}
              >
                {token.title || "Custom Token"}
              </HeadingMedium>
            </Box>
          </Box>
          <Box
            $align="center"
            $justify="flex-end"
            $gap="8px"
            $style={{
              flexDirection: "row",
            }}
          >
            <ParagraphSmall
              $style={{
                color: COLORS.gray50,
                textAlign: "right",
                whiteSpace: "nowrap",
              }}
            >
              {token.rightText}
            </ParagraphSmall>
            {token.address !== "" && (
              <Icon
                src={reduceOne}
                alt="Hide token"
                size={16}
                iconSize="16"
                background="transparent"
                pointer={true}
                onClick={() => {
                  hideToken(token.address);
                }}
              />
            )}
          </Box>
        </Box>
      ))}

      {hiddenTokens.length < 1 ? null : (
        <hr
          style={{
            borderTop: "1px solid #444444",
            borderBottom: "none",
            borderLeft: "none",
            borderRight: "none",
            margin: "6px 0",
          }}
        />
      )}

      {hiddenTokens.map((token, index) => (
        <Box
          key={`${token.title}-${index}`}
          $align="center"
          $justify="space-between"
          $style={{
            flexDirection: "row",
            width: "100%",
            padding: "5px",
          }}
        >
          <Box
            $align="center"
            $gap="8px"
            $style={{
              flexDirection: "row",
            }}
          >
            <Icon
              src={token.icon}
              alt={`${token.title} Icon`}
              size={64}
              iconSize="44"
              background="transparent"
            />
            <Box $align="flex-start" $style={{ flexDirection: "column", maxWidth: "200px" }}>
              <HeadingMedium
                $style={{
                  color: COLORS.gray50,
                  whiteSpace: "nowrap",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                }}
              >
                {token.title || "Custom Token"}
              </HeadingMedium>
            </Box>
          </Box>
          <Box
            $align="center"
            $justify="flex-end"
            $gap="8px"
            $style={{
              flexDirection: "row",
            }}
          >
            <ParagraphSmall
              $style={{
                color: COLORS.gray50,
                textAlign: "right",
                whiteSpace: "nowrap",
              }}
            >
              {token.rightText}
            </ParagraphSmall>
            <Icon
              src={addOne}
              alt="Show token"
              size={16}
              iconSize="16"
              background="transparent"
              pointer={true}
              onClick={() => {
                showToken(token.address);
              }}
            />
          </Box>
        </Box>
      ))}

      <Box $style={{ margin: "30px" }} />

      <Box
        $style={{
          position: "absolute",
          bottom: "24px",
          zIndex: 100,
          width: "87%",
        }}
      >
        <Button
          onClick={addCustomTokenNavigate}
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
          {t("wallet.manageTokens.manageTokensPage.addButton")}
        </Button>
      </Box>
    </Box>
  );
};
