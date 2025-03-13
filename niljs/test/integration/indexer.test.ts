import {newIndexerClient} from "./helpers.js";
import {addHexPrefix} from "../../src/index.js";
import {defaultAddress} from "../mocks/address.js";

const indexerClient = newIndexerClient();

test("getAddressActions", async () => {
  const actions = await indexerClient.getAddressActions(addHexPrefix(defaultAddress), 0);

  expect(Object.keys(actions).length).toBeGreaterThan(0);
});
