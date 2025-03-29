import {
  BUTTON_KIND,
  Button,
  COLORS,
  ChevronDownIcon,
  ChevronUpIcon,
  MonoParagraphMedium,
  NOTIFICATION_KIND,
  Notification,
  ParagraphMedium,
  PlusIcon,
  StatefulTooltip,
} from "@nilfoundation/ui-kit";
import type { AbiFunction } from "abitype";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { $balanceToken } from "../../../account-connector/model";
import { transactionRoute } from "../../../routing";
import { Link, Marker } from "../../../shared";
import { TokenInput, getTokenSymbolByAddress } from "../../../tokens";
import {
  $valueInputs,
  addValueInput,
  callMethod,
  sendMethod,
  setValueInput,
  toggleActiveKey,
} from "../../models/base.ts";
import { $callParamsValidationErrors } from "../../models/callParamsValidation.ts";
import type { CallParams } from "../../types.ts";
import { getResultDisplay } from "../../utils.tsx";
import { MethodInput } from "./MethodInput";
import { RemoveTokenButton } from "./RemoveTokenButton.tsx";
import { Result } from "./Result";

export type MethodProps = {
  func: AbiFunction;
  isOpen: boolean;
  error?: string;
  result?: unknown;
  loading?: boolean;
  txHash?: string;
  params?: CallParams[string];
  paramsHandler: (params: {
    functionName: string;
    paramName: string;
    value:
      | string
      | boolean
      | {
          type: string;
          value: string;
        }[];
    type: string;
  }) => void;
};

type MethodType = "Read" | "Write" | "Payable";

const getMethodType = (func: AbiFunction): MethodType => {
  if (func.stateMutability === "payable") {
    return "Payable";
  }

  if (func.stateMutability === "view" || func.stateMutability === "pure") {
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
  const [tokenBalance, valueInputs, errors] = useUnit([
    $balanceToken,
    $valueInputs,
    $callParamsValidationErrors,
  ]);
  const availableTokens = [
    { token: "NIL" },
    ...Object.keys(tokenBalance ?? {}).map((token) => ({
      token: getTokenSymbolByAddress(token),
    })),
  ];

  const methodType = getMethodType(func);
  const markerColor = getMarkerColor(methodType);
  const handler = methodType === "Read" ? callMethod : sendMethod;
  const functionName = func.name;
  const functionValues = valueInputs.find((v) => v.functionName === functionName)?.values;
  const methodErrors = errors[functionName] ?? {};

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
              {functionValues?.map((valueInput, index) => {
                const usedTokens = functionValues.map((v) => v.token);

                const availableInputTokens = availableTokens.filter(
                  (c) => !usedTokens.includes(c.token) || c.token === valueInput.token,
                );

                return (
                  // biome-ignore lint/correctness/useJsxKeyInIterable: can be the same for now
                  <div
                    className={css({
                      display: "flex",
                      flexDirection: "row",
                      alignItems: "center",
                      width: "100%",
                      gap: "8px",
                    })}
                  >
                    <TokenInput
                      className={css({
                        width: "100%",
                      })}
                      disabled={loading}
                      tokens={availableInputTokens}
                      onChange={({ amount, token }) => {
                        setValueInput({ index, amount, token, functionName });
                      }}
                      value={valueInput}
                    />
                    <RemoveTokenButton
                      index={index}
                      kind={BUTTON_KIND.secondary}
                      functionName={functionName}
                    />
                  </div>
                );
              })}
              <Button
                onClick={() => {
                  addValueInput({
                    availableTokens: availableTokens.map((c) => c.token),
                    functionName,
                  });
                }}
                disabled={
                  functionValues
                    ? functionValues.map((v) => v.token).length >= availableTokens.length
                    : false
                }
                kind={BUTTON_KIND.tertiary}
                overrides={{
                  Root: {
                    style: () => ({
                      width: "100%",
                      height: "48px",
                    }),
                  },
                }}
              >
                <PlusIcon size={26} /> Add token
              </Button>
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
                error={methodErrors[key]}
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
                {getResultDisplay(result)}
              </ParagraphMedium>
            </Result>
          )}
          {txHash && (
            <Notification
              kind={NOTIFICATION_KIND.positive}
              overrides={{
                Body: {
                  style: {
                    marginLeft: 0,
                    marginRight: 0,
                    width: "100%",
                  },
                },
              }}
            >
              Transaction sent with hash{" "}
              <Link
                to={transactionRoute}
                params={{ hash: txHash }}
                className={css({
                  wordBreak: "break-all",
                })}
                target="_blank"
              >
                {txHash}
              </Link>
            </Notification>
          )}
          {error && (
            <Notification
              overrides={{
                Body: {
                  style: {
                    marginLeft: 0,
                    marginRight: 0,
                    width: "100%",
                    wordBreak: "break-all",
                    overflowWrap: "break-word",
                  },
                },
              }}
              kind={NOTIFICATION_KIND.negative}
            >
              {error}
            </Notification>
          )}
        </div>
      )}
    </div>
  );
};
