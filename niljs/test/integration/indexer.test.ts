import {newIndexerClient} from "./helpers.js";
import {addHexPrefix} from "../../src/index.js";
import {defaultAddress} from "../mocks/address.js";

const indexerClient = newIndexerClient();

test("getAddressActions", async () => {

  const actions = await indexerClient.getAddressActions("0x0000222222222222222222222222222222222222", 0);

  expect(Object.keys(actions).length).toBeGreaterThan(0);
});
