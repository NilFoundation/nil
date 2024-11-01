import {
  BUTTON_KIND,
  Button,
  COLORS,
  ChevronDownIcon,
  ChevronUpIcon,
  NOTIFICATION_KIND,
  ParagraphMedium,
  Notification,
  MonoParagraphMedium,
  StatefulTooltip,
} from "@nilfoundation/ui-kit";
import type { AbiFunction } from "abitype";
import { useStyletron } from "baseui";
import { toggleActiveKey, $valueInput, setValueInput, callMethod, sendMethod } from "../../model";
import { Link, Marker } from "../../../shared";
import { transactionRoute } from "../../../routing";
import { MethodInput } from "./MethodInput";
import { CurrencyInput, getCurrencySymbolByAddress } from "../../../currencies";
import { $balanceCurrency } from "../../../account-connector/models/model";
import { useUnit } from "effector-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { Result } from "./Result";

export type MethodProps = {
  func: AbiFunction;
  isOpen: boolean;
  error?: string;
  result?: unknown;
  loading?: boolean;
  txHash?: string;
  params?: Record<string, unknown>;
  paramsHandler: (params: {
    functionName: string;
    paramName: string;
    value: unknown;
  }) => void;
};

type MethodType = "Read" | "Write" | "Payable";

const getMethodType = (func: AbiFunction): MethodType => {
  if (func.stateMutability === "payable") {
    return "Payable";
  }

  if (func.stateMutability === "view") {
    return "Read";
  }

  return "Write";
};

const getMarkerColor = (type: MethodType) => {
  if (type === "Read") {
    return COLORS.blue200;
  }

  if (type === "Write") {
    return COLORS.gray50;
  }

  if (type === "Payable") {
    return COLORS.red200;
  }
};

export const Method = ({
  func,
  isOpen,
  error,
  result,
  loading,
  txHash,
  paramsHandler,
  params,
}: MethodProps) => {
  const [css] = useStyletron();
  const [currecyBalance, valueInput] = useUnit([$balanceCurrency, $valueInput]);
  const availiableCurencies = [
    { currency: "NIL" },
    ...Object.keys(currecyBalance ?? {}).map((currency) => ({
      currency: getCurrencySymbolByAddress(currency),
    })),
  ];

  const methodType = getMethodType(func);
  const markerColor = getMarkerColor(methodType);
  const handler = methodType === "Read" ? callMethod : sendMethod;

  return (
    <div
      key={func.name}
      className={css({
        borderTop: `1px solid ${COLORS.gray800}`,
      })}
    >
      <Button
        kind={BUTTON_KIND.text}
        endEnhancer={isOpen ? <ChevronUpIcon /> : <ChevronDownIcon />}
        onClick={() => {
          toggleActiveKey(func.name);
        }}
        overrides={{
          Root: {
            style: {
              width: "100%",
              paddingLeft: "0",
              paddingRight: "8px",
              display: "flex",
              justifyContent: "space-between",
              textDecoration: "none",
            },
          },
        }}
      >
        <div
          className={css({
            display: "flex",
            gap: "8px",
          })}
        >
          <StatefulTooltip
            showArrow={false}
            content={`${methodType} function`}
            popoverMargin={4}
            placement="bottom"
          >
            <div
              className={css({
                ...expandProperty("padding", "8px"),
              })}
            >
              <Marker $color={markerColor} />
            </div>
          </StatefulTooltip>
          {`${func.name} ()`}
        </div>
      </Button>
      {isOpen && (
        <div
          className={css({
            display: "flex",
            flexDirection: "column",
            gap: "8px",
            paddingBottom: "8px",
          })}
        >
          {methodType === "Payable" && (
            <div
              className={css({
                display: "flex",
                flexDirection: "column",
                alignItems: "flex-start",
                justifyContent: "center",
              })}
            >
              <CurrencyInput
                className={css({
                  width: "100%",
                })}
                disabled={loading}
                caption="Tokens"
                currencies={availiableCurencies}
                onChange={({ amount, currency }) => {
                  setValueInput({ amount, currency });
                }}
                value={valueInput}
              />
            </div>
          )}
          {func.inputs.map((input, index) => {
            const key = input.name || `${index}`;
            return (
              <MethodInput
                key={key}
                methodName={func.name}
                paramsHandler={paramsHandler}
                params={params}
                paramName={key}
                input={input}
              />
            );
          })}
          <Button
            onClick={() => {
              handler(func.name);
            }}
            isLoading={loading}
            disabled={loading}
            overrides={{
              Root: {
                style: ({ $disabled }) => ({
                  backgroundColor: $disabled ? `${COLORS.gray400}!important` : "",
                  width: "100%",
                  height: "48px",
                }),
              },
            }}
          >
            {methodType === "Read" ? "Get" : "Submit"}
          </Button>
          {result !== undefined && (
            <Result>
              <MonoParagraphMedium
                color={COLORS.gray200}
                className={css({
                  wordBreak: "break-all",
                  marginBottom: "8px",
                })}
              >
                Result:
              </MonoParagraphMedium>
              <ParagraphMedium
                className={css({
                  wordBreak: "break-all",
                })}
              >
                {String(result)}
              </ParagraphMedium>
            </Result>
          )}
          {txHash && (
            <Notification kind={NOTIFICATION_KIND.positive}>
              Transaction sent with hash{" "}
              <Link
                to={transactionRoute}
                params={{ hash: txHash }}
                className={css({
                  wordBreak: "break-all",
                })}
              >
                {txHash}
              </Link>
            </Notification>
          )}
          {error && <Notification kind={NOTIFICATION_KIND.negative}>{error}</Notification>}
        </div>
      )}
    </div>
  );
};
