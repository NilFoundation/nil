import { COLORS, FormControl, Input, Select } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import type { Currency } from "./Currency";

type CurrencyInputProps = {
  value: { currency: string | Currency; amount: string };
  onChange: (value: { currency: string | Currency; amount: string }) => void;
  currencies: { currency: string | Currency }[];
  className?: string;
  label?: string;
  disabled?: boolean;
  caption?: string;
};

const CurrencyInput: FC<CurrencyInputProps> = ({
  value,
  onChange,
  currencies,
  className,
  label,
  disabled = false,
  caption,
}) => {
  const [css] = useStyletron();

  return (
    <div className={`${css({})} ${className}`}>
      <FormControl label={label} caption={caption}>
        <Input
          disabled={disabled}
          overrides={{
            Input: {
              style: {
                "::-webkit-outer-spin-button": {
                  WebkitAppearance: "none",
                  margin: 0,
                },
                "::-webkit-inner-spin-button": {
                  WebkitAppearance: "none",
                  margin: 0,
                },
                "-moz-appearance": "textfield",
              },
            },
            Root: {
              style: {
                backgroundColor: COLORS.gray700,
                ":hover": {
                  backgroundColor: COLORS.gray600,
                },
              },
            },
          }}
          type="number"
          value={value.amount}
          onChange={(e) => {
            onChange({
              currency: value.currency,
              amount: e.currentTarget.value,
            });
          }}
          endEnhancer={
            <Select
              disabled={disabled}
              options={[...currencies.map(({ currency }) => ({
                label: currency,
                id: currency,
              }))]}
              searchable={false}
              overrides={{
                ControlContainer: {
                  style: {
                    width: "100px",
                    backgroundColor: "transparent",
                    boxShadow: "none",
                    ":has(input:focus-within)": {
                      boxShadow: "none",
                    },
                    ":hover": {
                      backgroundColor: "transparent",
                    },
                  },
                },
              }}
              placeholder=""
              clearable={false}
              onChange={(params) => {
                onChange({
                  currency: params.value[0].label as string,
                  amount: value.amount,
                });
              }}
              value={[
                {
                  label: value.currency,
                  id: value.currency,
                },
              ]}
            />
          }
        />
      </FormControl>
    </div>
  );
};

export { CurrencyInput };
