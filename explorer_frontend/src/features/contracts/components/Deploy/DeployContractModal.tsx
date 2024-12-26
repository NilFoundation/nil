import {
  Modal,
  ModalHeader,
  ModalBody,
  LabelLarge,
  Tabs,
  Tab,
  TAB_KIND,
} from "@nilfoundation/ui-kit";
import {} from "../../models/base";
import {} from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { DeployTab } from "./DeployTab";
import { AssignTab } from "./AssignTab";
import type { TabsOverrides } from "baseui/tabs";
import { useStore } from "effector-react";
import { setActiveComponent, $activeComponent } from "../../models/base";
import { ActiveComponent } from "./ActiveComponent";

type DeployContractModalProps = {
  onClose?: () => void;
  isOpen?: boolean;
  name: string;
};

export const DeployContractModal: FC<DeployContractModalProps> = ({ onClose, isOpen, name }) => {
  const activeComponent = useStore($activeComponent) || ActiveComponent.Deploy;

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
          activeKey={activeComponent}
          overrides={tabsOverrides}
          onChange={({ activeKey }) => setActiveComponent(activeKey as ActiveComponent)}
        >
          <Tab
            title="Deploy"
            key={ActiveComponent.Deploy}
            kind={TAB_KIND.primary}
            onClick={() => setActiveComponent(ActiveComponent.Deploy)}
          >
            <DeployTab />
          </Tab>
          <Tab
            title="Assign address"
            kind={TAB_KIND.primary}
            key={ActiveComponent.Assign}
            onClick={() => setActiveComponent(ActiveComponent.Assign)}
          >
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
