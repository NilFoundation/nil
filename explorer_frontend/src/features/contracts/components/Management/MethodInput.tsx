import { FormControl, Input, LabelMedium } from "@nilfoundation/ui-kit";
import type { AbiInternalType, AbiParameter } from "abitype";
import { useStyletron } from "baseui";
import type { FC } from "react";

type MethodInputProps = {
  input: AbiParameter;
  methodName: string;
  paramName: string;
  params?: Record<string, unknown>;
  paramsHandler: (params: { functionName: string; paramName: string; value: unknown }) => void;
};

const isAbiParameterTuple = (
  input: AbiParameter,
): input is {
  type: "tuple" | `tuple[${string}]`;
  name?: string | undefined;
  internalType?: AbiInternalType | undefined;
  components: readonly AbiParameter[];
} => {
  return input.type === "tuple";
};

const MethodInput: FC<MethodInputProps> = ({
  input,
  params,
  paramsHandler,
  methodName,
  paramName,
}: MethodInputProps) => {
  const { type, name } = input;
  const [css, theme] = useStyletron();

  const extractInputValue = (val: unknown) => {
    if (typeof val === "object" && val !== null && "value" in val) {
      return val.value;
    }
    return val;
  };

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
              const componentValue = params ? (params[paramName] as AbiParameter) : "";
              const inputValue = componentValue ? componentValue.name : "";

              return (
                // biome-ignore lint/suspicious/noArrayIndexKey: <explanation>
                <div key={i}>
                  <FormControl label={name} caption={type}>
                    <Input
                      overrides={{
                        Root: {
                          style: {
                            backgroundColor:
                              theme.colors.inputButtonAndDropdownOverrideBackgroundColor,
                            ":hover": {
                              backgroundColor:
                                theme.colors.inputButtonAndDropdownOverrideBackgroundHoverColor,
                            },
                          },
                        },
                      }}
                      value={inputValue}
                      onChange={(e) => {
                        const value = e.target.value;
                        const mergedValue = { ...componentValue, [name]: value };
                        paramsHandler({ functionName: methodName, paramName, value: mergedValue });
                      }}
                      placeholder={type === "address" ? "0x..." : ""}
                    />
                  </FormControl>
                </div>
              );
            })}
          </div>
        </>
      ) : (
        <FormControl label={name} caption={type}>
          <Input
            overrides={{
              Root: {
                style: {
                  backgroundColor: theme.colors.inputButtonAndDropdownOverrideBackgroundColor,
                  ":hover": {
                    backgroundColor:
                      theme.colors.inputButtonAndDropdownOverrideBackgroundHoverColor,
                  },
                },
              },
            }}
            value={params?.[paramName] ? String(extractInputValue(params[paramName])) : ""}
            onChange={(e) => {
              const value = e.target.value;
              paramsHandler({ functionName: methodName, paramName, value });
            }}
          />
        </FormControl>
      )}
    </div>
  );
};

export { MethodInput };
