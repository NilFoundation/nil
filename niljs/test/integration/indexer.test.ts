import { newIndexerClient } from "./helpers.js";

const indexerClient = newIndexerClient();

test.skip("getAddressActions", async () => {
  const actions = await indexerClient.getAddressActions(
    "0x0000222222222222222222222222222222222222",
    0,
  );

  expect(Object.keys(actions).length).toBeGreaterThan(0);
});
