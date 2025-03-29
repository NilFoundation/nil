import type { Hex } from "@nilfoundation/niljs";
import { Button, COLORS, FormControl, Input, LabelSmall, SPACE } from "@nilfoundation/ui-kit";
import { INPUT_KIND, INPUT_SIZE } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { InputOverrides } from "baseui/input";
import { useUnit } from "effector-react";
import { $smartAccount } from "../../../account-connector/model";
import {
  $activeAppWithState,
  $importedSmartContractAddress,
  $importedSmartContractAddressError,
  $importedSmartContractAddressIsValid,
  importSmartContract,
  importSmartContractFx,
  setImportedSmartContractAddress,
} from "../../models/base";

export const ImportContractTab = () => {
  const [smartAccount, pending, importedAddress, activeApp, addressIsValid, errorMessage] = useUnit(
    [
      $smartAccount,
      importSmartContractFx.pending,
      $importedSmartContractAddress,
      $activeAppWithState,
      $importedSmartContractAddressIsValid,
      $importedSmartContractAddressError,
    ],
  );

  const [css, theme] = useStyletron();

  const inputOverrides: InputOverrides = {
    Root: {
      style: () => ({
        backgroundColor: theme.colors.backgroundSecondary,
        ":hover": {
          backgroundColor: theme.colors.backgroundTertiary,
        },
      }),
    },
  };

  return (
    <>
      <div
        className={css({
          flexGrow: 0,
          paddingBottom: SPACE[16],
        })}
      >
        <FormControl label="Address">
          <Input
            kind={INPUT_KIND.secondary}
            size={INPUT_SIZE.small}
            overrides={inputOverrides}
            onChange={(e) => {
              const value = e.target.value as Hex;
              setImportedSmartContractAddress(value);
            }}
            value={importedAddress}
            placeholder="0x"
          />
        </FormControl>
        {errorMessage && (
          <LabelSmall
            className={css({
              color: COLORS.red400,
              marginTop: SPACE[4],
            })}
          >
            {errorMessage}
          </LabelSmall>
        )}
        <LabelSmall
          className={css({
            color: COLORS.gray400,
            marginTop: SPACE[4],
          })}
        >
          Import the already deployed {activeApp?.name} smart contract using its address.
        </LabelSmall>
      </div>
      <div>
        <Button
          onClick={() => {
            importSmartContract();
          }}
          isLoading={pending}
          disabled={pending || !smartAccount || !addressIsValid}
        >
          Import
        </Button>
      </div>
    </>
  );
};
