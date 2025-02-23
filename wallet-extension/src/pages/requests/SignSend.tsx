import { convertEthToWei } from "@nilfoundation/niljs";
import {
  Button,
  COLORS,
  HeadingMedium,
  ParagraphSmall,
  ParagraphXSmall,
} from "@nilfoundation/ui-kit";
import { useStore } from "effector-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import websiteIcon from "../../../public/icons/website.svg";
import { sendTransaction } from "../../features/blockchain/smartAccount.ts";
import { Box, Icon, InputErrorMessage } from "../../features/components/shared";
import { $balance, $balanceCurrency, $smartAccount } from "../../features/store/model";
import { convertWeiToEth, formatAddress } from "../../features/utils";

// Mock storage functions for transaction data
const getFromStorage = (key) => {
  return new Promise((resolve) => {
    chrome.storage.local.get([key], (result) => {
      resolve(result[key]);
    });
  });
};

export const SignSend = () => {
  const { t } = useTranslation("translation");
  const smartAccount = useStore($smartAccount);
  const balance = useStore($balance);
  const balanceCurrencies = useStore($balanceCurrency);

  const [requestId, setRequestId] = useState<string>("");
  const [origin, setOrigin] = useState<string>("");
  const [transactionDetails, setTransactionDetails] = useState(null);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [valueError, setValueError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tokenErrors, setTokenErrors] = useState<Record<string, string>>({});

  // Fetch transaction data
  useEffect(() => {
    const fetchTransactionData = async () => {
      const hash = window.location.hash;
      const queryString = hash.split("?")[1];
      const urlParams = new URLSearchParams(queryString);

      const requestIdParam = urlParams.get("requestId");
      const originParam = urlParams.get("origin");

      if (requestIdParam) setRequestId(requestIdParam);
      if (originParam) setOrigin(decodeURIComponent(originParam));

      if (requestIdParam) {
        const txData = await getFromStorage(`tx_${requestIdParam}`);
        if (txData) {
          setTransactionDetails(txData);
          validateTransaction(txData);
        }
      }
    };

    fetchTransactionData();
  }, [balance, balanceCurrencies]);

  // Validate transaction fields
  const validateTransaction = (tx) => {
    setValueError(null);
    setTokenErrors({});

    // Validate value
    if (tx.value) {
      const valueInWei = convertEthToWei(tx.value);
      if (balance && valueInWei > balance) {
        setValueError(`Insufficient balance. You have ${convertWeiToEth(balance, 8)} Nil`);
      }
    }

    // Validate tokens
    if (balanceCurrencies && tx.tokens?.length) {
      const tokenErrorsMap: Record<string, string> = {};
      for (const token of tx.tokens) {
        const tokenKey = `${token.id}_${token.amount}`;

        const tokenBalance = balanceCurrencies[token.id] ?? BigInt(0);
        if (!balanceCurrencies[token.id]) {
          tokenErrorsMap[tokenKey] = "Token not found";
        } else if (BigInt(token.amount) > tokenBalance) {
          tokenErrorsMap[tokenKey] = `Insufficient balance for ${token.amount}`;
        }
      }
      setTokenErrors(tokenErrorsMap);
    }
  };

  // Handle cancel action
  const handleCancel = () => {
    const connectPort = chrome.runtime.connect({ name: "signsend-request" });
    connectPort.postMessage({ requestId, origin });

    // Clean up storage and close window
    chrome.storage.local.remove(`tx_${requestId}`);
    setTimeout(() => window.close(), 100);
  };

  // Handle confirm action
  const handleConfirm = async () => {
    if (!smartAccount || !transactionDetails) {
      setError("Missing smart account or transaction details");
      return;
    }

    if (valueError || Object.keys(tokenErrors).length > 0) {
      setError("Cannot proceed due to validation errors");
      return;
    }

    setIsLoading(true);

    try {
      const receiptHash = await sendTransaction({
        smartAccount,
        transactionParams: transactionDetails,
      });

      const connectPort = chrome.runtime.connect({ name: "signsend-request" });
      connectPort.postMessage({ requestId, origin, receiptHash });

      // Clean up storage and close window
      chrome.storage.local.remove(`tx_${requestId}`);
      setTimeout(() => window.close(), 100);
    } catch (err) {
      setError("Failed to send transaction check logs");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box $$justify="space-between" $padding="24px" $style={{ height: "100vh" }}>
      {/* Title */}
      <HeadingMedium $style={{ color: COLORS.gray50, marginBottom: "24px" }}>
        {t("requests.sendSign.title")}
      </HeadingMedium>

      {/* Website Section */}
      <Box
        $align="flex-start"
        $direction="column"
        $gap="8px"
        $style={{ width: "100%", marginBottom: "12px" }}
      >
        <ParagraphSmall $style={{ color: COLORS.gray200 }}>
          {t("requests.sendSign.requestedLabel")}
        </ParagraphSmall>
        <Box $align="center" $gap="12px" $style={{ flexDirection: "row" }}>
          <Icon src={websiteIcon} alt="Website Icon" size={64} iconSize="100%" />
          <Box>
            <HeadingMedium>{origin || "Unknown"}</HeadingMedium>
            <ParagraphXSmall $style={{ color: COLORS.gray200 }}>{origin}</ParagraphXSmall>
          </Box>
        </Box>
      </Box>

      {/* Divider Line */}
      <Box $style={{ height: "2px", backgroundColor: COLORS.gray700, marginBottom: "12px" }} />

      {/* Transaction Details Section */}
      {transactionDetails && (
        <Box $direction="column" $gap="8px" $style={{ width: "100%" }}>
          {/* From and To */}
          {smartAccount?.address && (
            <Box $style={{ justifyContent: "space-between", flexDirection: "row" }}>
              <ParagraphSmall $style={{ color: COLORS.gray200 }}>From:</ParagraphSmall>
              <ParagraphSmall $style={{ color: COLORS.gray50 }}>
                {formatAddress(smartAccount.address)}
              </ParagraphSmall>
            </Box>
          )}

          {transactionDetails.to && (
            <Box $style={{ justifyContent: "space-between", flexDirection: "row" }}>
              <ParagraphSmall $style={{ color: COLORS.gray200 }}>To:</ParagraphSmall>
              <ParagraphSmall $style={{ color: COLORS.gray50 }}>
                {formatAddress(transactionDetails.to)}
              </ParagraphSmall>
            </Box>
          )}

          {/* Value Field with Error Display */}
          {transactionDetails.value !== 0 && (
            <Box>
              <Box $style={{ justifyContent: "space-between", flexDirection: "row" }}>
                <ParagraphSmall $style={{ color: COLORS.gray200 }}>Value:</ParagraphSmall>
                <ParagraphSmall $style={{ color: COLORS.gray50 }}>
                  {`${transactionDetails.value} Nil`}
                </ParagraphSmall>
              </Box>
              {valueError && <InputErrorMessage error={valueError} style={{ marginTop: "3px" }} />}
            </Box>
          )}

          {/* Tokens Field with Per-Token Error Display */}
          {transactionDetails.tokens?.length > 0 && (
            <Box
              $style={{
                maxHeight: "150px",
                overflowY: "auto",
                scrollbarWidth: "thin",
                scrollbarColor: `${COLORS.gray600} ${COLORS.gray800}`,
              }}
            >
              {transactionDetails.tokens.map((token) => (
                <Box key={token.id}>
                  <Box $style={{ justifyContent: "space-between", flexDirection: "row" }}>
                    <ParagraphSmall $style={{ color: COLORS.gray200 }}>
                      Token ({formatAddress(token.id)}):
                    </ParagraphSmall>
                    <ParagraphSmall $style={{ color: COLORS.gray50 }}>
                      {`${token.amount}`}
                    </ParagraphSmall>
                  </Box>
                  {tokenErrors[`${token.id}_${token.amount}`] && (
                    <InputErrorMessage
                      error={tokenErrors[`${token.id}_${token.amount}`]}
                      style={{ marginTop: "3px" }}
                    />
                  )}
                </Box>
              ))}
            </Box>
          )}

          {/* Data (Contract Interaction) */}
          {transactionDetails.data && (
            <Box $style={{ justifyContent: "space-between", flexDirection: "row" }}>
              <ParagraphSmall $style={{ color: COLORS.gray200 }}>Data:</ParagraphSmall>
              <ParagraphSmall $style={{ color: COLORS.gray50 }}>
                Contract Interaction
              </ParagraphSmall>
            </Box>
          )}
        </Box>
      )}

      {/* Empty space to separate sections */}
      <Box $style={{ flexGrow: 1 }} />

      {/* Buttons */}
      <Box $align="center" $gap="8px" $style={{ width: "100%" }}>
        {error && (
          <InputErrorMessage error={error} style={{ marginTop: "3px", marginBottom: "12px" }} />
        )}

        <Button
          onClick={handleConfirm}
          isLoading={isLoading}
          disabled={!!valueError || Object.keys(tokenErrors).length > 0}
          overrides={{
            Root: { style: { width: "100%", height: "48px" } },
          }}
        >
          {t("requests.sendSign.confirmButton")}
        </Button>

        <Button
          onClick={handleCancel}
          overrides={{
            Root: {
              style: {
                width: "100%",
                height: "48px",
                backgroundColor: COLORS.gray800,
                color: COLORS.gray200,
                ":hover": { backgroundColor: COLORS.gray700 },
              },
            },
          }}
        >
          {t("requests.sendSign.cancelButton")}
        </Button>
      </Box>
    </Box>
  );
};
