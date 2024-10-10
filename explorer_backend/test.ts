import { fetchLatestBlocks } from "./daos/blocks";

fetchLatestBlocks(0, 10).then((res) => {
  console.log(res);
});
