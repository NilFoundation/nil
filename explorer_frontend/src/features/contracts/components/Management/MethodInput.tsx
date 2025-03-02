import { COLORS, FormControl, Input, LabelMedium } from "@nilfoundation/ui-kit";
import type { AbiParameter } from "abitype";
import type { FormControlOverrides } from "baseui/form-control";
import type { InputOverrides } from "baseui/input";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import type { CallParams } from "../../types";
import { isAbiParameterTuple } from "../../utils";

type MethodInputProps = {
  input: AbiParameter;
  methodName: string;
  paramName: string;
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
  error?: string | null | Record<string, string | null>;
};

const formControlOverries: FormControlOverrides = {
  Caption: {
    style: ({ $error }) => ({
      marginTop: "8px",
      ...($error ? { color: COLORS.red200 } : {}),
    }),
  },
};

const inputOverrides: InputOverrides = {
  Root: {
    style: ({ $error }) => ({
      ...($error
        ? { boxShadow: `0px 0px 0px 2px ${COLORS.gray900}, 0px 0px 0px 4px ${COLORS.red200}` }
        : {}),
    }),
  },
  Input: {
    style: ({ $error }) => ({
      ...($error ? { color: COLORS.red200 } : {}),
    }),
  },
};

const MethodInput: FC<MethodInputProps> = ({
  input,
  params,
  paramsHandler,
  methodName,
  paramName,
  error,
}: MethodInputProps) => {
  const { type, name } = input;
  const [css] = useStyletron();

  return (
    <div>
      {isAbiParameterTuple(input) ? (
        <>
          <LabelMedium>{name}</LabelMedium>
          <div
            className={css({
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gridTemplateRows: "1fr",
              gap: "8px",
            })}
          >
            {input.components.map(({ name = "", type }, i) => {
              const inputValue = params?.[paramName]
                ? (params[paramName].value as {
                    type: string;
                    value: string;
                  }[])
                : [];
              const componentValue = inputValue[i] ?? { type, value: "" };
              const errs = error as Record<string, string | null> | null;
              const err = errs ? errs[i.toString()] : null;

              console.log("ERR", error, err);

              return (
                // biome-ignore lint/suspicious/noArrayIndexKey: <explanation>
                <div key={i}>
                  <FormControl
                    label={name}
                    caption={type}
                    overrides={formControlOverries}
                    error={err}
                  >
                    <Input
                      value={componentValue.value}
                      onChange={(e) => {
                        const value = e.target.value;
                        const mergedValue = [...inputValue];
                        mergedValue[i] = { type, value };
                        paramsHandler({
                          functionName: methodName,
                          paramName,
                          value: mergedValue,
                          type: input.type,
                        });
                      }}
                      placeholder={type === "address" ? "0x..." : ""}
                      overrides={inputOverrides}
                    />
                  </FormControl>
                </div>
              );
            })}
          </div>
        </>
      ) : (
        <>
          <FormControl
            label={name}
            caption={type}
            error={error as string | null}
            overrides={formControlOverries}
          >
            <Input
              value={params?.[paramName] ? String(params[paramName].value) : ""}
              onChange={(e) => {
                const value = e.target.value;
                paramsHandler({ functionName: methodName, paramName, value, type: input.type });
              }}
              placeholder={type === "address" ? "0x..." : ""}
              overrides={inputOverrides}
            />
          </FormControl>
        </>
      )}
    </div>
  );
};

export { MethodInput };
