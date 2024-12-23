import { 
  Modal, 
  ModalHeader, 
  ModalBody,
  LabelLarge, 
  Tabs, 
  Tab, 
  TAB_KIND 
} from "@nilfoundation/ui-kit";
import {} from "../../models/base";
import {} from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { DeployTab } from "./DeployTab";
import { AssignTab } from "./AssignTab";
import { useState } from "react";
import type { TabsOverrides } from "baseui/tabs";

type DeployContractModalProps = {
  onClose?: () => void;
  isOpen?: boolean;
  name: string;
};

export const DeployContractModal: FC<DeployContractModalProps> = ({ onClose, isOpen, name }) => {
  const [activeKey, setActiveKey] = useState("deploy");

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
            height: "557px",
          },
        },
      }}
    >
      <ModalHeader>
        <LabelLarge>{name}</LabelLarge>
      </ModalHeader>

      <ModalBody>
        <Tabs
          activeKey={activeKey}
          overrides={tabsOverrides}
          onChange={({ activeKey }) => setActiveKey(activeKey)}
        >
          <Tab title="Deploy" key="deploy" kind={TAB_KIND.primary}>
            <DeployTab />
          </Tab>
          <Tab title="Assign address" key="assign" kind={TAB_KIND.primary}>
            <AssignTab />
          </Tab>
        </Tabs>
      </ModalBody>
    </Modal>
  );
};

const tabsOverrides: TabsOverrides = {
  TabContent: {
    style: {
      paddingLeft: 0,
      paddingRight: 0,
    },
  },
  TabBar: {
    style: {
      paddingLeft: 100,
      paddingRight: 0,
    },
  },
  Tab: {
    style: {
      fontSize: "16px",
    },
  },
};
