import { TABS_ORIENTATION, Tab, Tabs } from "@nilfoundation/ui-kit";
import { useState } from "react";
import { Box } from "../../shared";
import { ActivityTab } from "./ActivityTab.tsx";
import { NFTTab } from "./NFTTab.tsx";
import { TokenTab } from "./TokenTab.tsx";

interface TabChangeEvent {
  activeKey: string;
}

export const ResourceTabs = () => {
  const [activeKey, setActiveKey] = useState("tokens");

  const handleTabChange = (key: TabChangeEvent) => {
    setActiveKey(key.activeKey);
  };

  return (
    <Box $style={{ width: "100%", padding: 0 }}>
      <Tabs
        orientation={TABS_ORIENTATION.horizontal}
        activeKey={activeKey}
        onChange={handleTabChange}
        overrides={{
          Root: {
            style: {
              display: "flex",
              justifyContent: "space-between",
              width: "100%",
              backgroundColor: "transparent",
            },
          },
          Tab: {
            style: {
              flex: "1",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              textAlign: "center",
              ":hover": {
                backgroundColor: "transparent",
              },
            },
          },
          TabContent: {
            style: {
              padding: "0px!important",
            },
          },
        }}
      >
        {/* Tokens Tab */}
        <Tab title="Tokens" key={"tokens"}>
          <TokenTab />
        </Tab>

        {/* NFTs Tab */}
        <Tab title="NFTs" key={"nft"}>
          <NFTTab />
        </Tab>

        {/* Activity Tab */}
        <Tab title="Activity" key="activity">
          <ActivityTab />
        </Tab>
      </Tabs>
    </Box>
  );
};
