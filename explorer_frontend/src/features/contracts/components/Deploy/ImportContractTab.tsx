import type { Hex } from "@nilfoundation/niljs";
import { Button, COLORS, FormControl, Input, LabelSmall, SPACE } from "@nilfoundation/ui-kit";
import { INPUT_KIND, INPUT_SIZE } from "@nilfoundation/ui-kit";
import type { InputOverrides } from "baseui/input";
import { useUnit } from "effector-react";
import { useStyletron } from "styletron-react";
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
import { isTutorialPage } from "../../../code/model";

export const ImportContractTab = () => {
  const [smartAccount, pending, importedAddress, activeApp, addressIsValid, errorMessage, isTutorial] = useUnit(
    [
      $smartAccount,
      importSmartContractFx.pending,
      $importedSmartContractAddress,
      $activeAppWithState,
      $importedSmartContractAddressIsValid,
      $importedSmartContractAddressError,
      isTutorialPage,
    ],
  );

  const [css] = useStyletron();

  const inputOverrides: InputOverrides = {
    Root: {
      style: () => ({
        backgroundColor: isTutorial ? COLORS.blue800 : COLORS.gray800,
        ":hover": {
          backgroundColor: isTutorial ? COLORS.blue700 : COLORS.gray700,
        }
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


