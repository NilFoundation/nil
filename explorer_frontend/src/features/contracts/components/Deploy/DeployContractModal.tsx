import {
  Button,
  COLORS,
  Checkbox,
  FormControl,
  Input,
  Modal,
  ModalBody,
  ModalHeader,
  SPACE,
  HeadingMedium,
  LabelLarge,
  ParagraphSmall,
} from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import {
  $deploymentArgs,
  $shardId,
  deploySmartContract,
  deploySmartContractFx,
  setDeploymentArg,
  setShardId,
} from "../../model";
import { useStyletron } from "styletron-react";
import { $constructor } from "../../init";
import type { FC } from "react";
import { $wallet } from "../../../account-connector/models/model";
import { ShardIdInput } from "./ShardIdInput";

type DeployContractModalProps = {
  onClose?: () => void;
  isOpen?: boolean;
  name: string;
};

export const DeployContractModal: FC<DeployContractModalProps> = ({ onClose, isOpen, name }) => {
  const [wallet, args, constuctorAbi, pending, shardId] = useUnit([
    $wallet,
    $deploymentArgs,
    $constructor,
    deploySmartContractFx.pending,
    $shardId,
  ]);
  const [css] = useStyletron();

  return (
    <Modal
      autoFocus={false}
      isOpen={isOpen}
      onClose={onClose}
      size="min(770px, 80vw)"
      overrides={{
        Dialog: {
          style: {
            paddingBottom: 0,
          },
        },
      }}
    >
      <ModalHeader>
        <LabelLarge>{name}</LabelLarge>
      </ModalHeader>
      <ModalBody>
        <div
          className={css({
            flexGrow: 0,
            paddingBottom: SPACE[16],
          })}
        >
          <FormControl
            label="Wallet"
            caption="From this wallet contract will be recorded to network"
          >
            <Input
              overrides={{
                Root: {
                  style: {
                    marginBottom: SPACE[8],
                  },
                },
              }}
              name="Wallet"
              value={wallet?.address ?? ""}
              disabled
              readOnly
            />
          </FormControl>
        </div>
        <div>
          <ShardIdInput shardId={shardId} setShardId={setShardId} disabled={pending} />
          {constuctorAbi?.inputs.length ? (
            <div
              className={css({
                paddingTop: "16px",
                borderTop: `1px solid ${COLORS.gray800}`,
                borderBottom: `1px solid ${COLORS.gray800}`,
                maxHeight: "30vh",
                marginBottom: "24px",
              })}
            >
              <HeadingMedium
                className={css({
                  marginBottom: SPACE[8],
                })}
              >
                Deployment arguments
              </HeadingMedium>
              {constuctorAbi.inputs.map((input) => {
                if (typeof input.name !== "string") {
                  return null;
                }
                const name = input.name;
                return (
                  <FormControl label={name} caption={input.type} key={name}>
                    {input.type === "bool" ? (
                      <Checkbox
                        overrides={{
                          Root: {
                            style: {
                              marginBottom: SPACE[8],
                            },
                          },
                        }}
                        key={name}
                        checked={typeof args[name] === "boolean" ? !!args[name] : false}
                        onChange={(e) => {
                          setDeploymentArg({ key: name, value: e.target.checked });
                        }}
                      />
                    ) : (
                      <Input
                        key={name}
                        overrides={{
                          Root: {
                            style: {
                              marginBottom: SPACE[8],
                            },
                          },
                        }}
                        name={name}
                        value={typeof args[name] === "string" ? `${args[name]}` : ""}
                        onChange={(e) => {
                          setDeploymentArg({ key: name, value: e.target.value });
                        }}
                      />
                    )}
                  </FormControl>
                );
              })}
            </div>
          ) : (
            <ParagraphSmall marginBottom="24px" color={COLORS.gray400}>
              No deployment arguments
            </ParagraphSmall>
          )}
          <Button
            onClick={() => {
              deploySmartContract();
            }}
            isLoading={pending}
            disabled={pending || !wallet || shardId === null}
          >
            Deploy
          </Button>
        </div>
      </ModalBody>
    </Modal>
  );
};
