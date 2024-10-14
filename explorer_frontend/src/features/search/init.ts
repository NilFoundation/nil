import { sample } from "effector";
import { addressRoute } from "../routing/routes/addressRoute";
import { transactionRoute } from "../routing/routes/transactionRoute";
import { blockRoute } from "../routing/routes/blockRoute";
import { isHex, removeHexPrefix } from "../shared/utils/hex";
import {
  $focused,
  $query,
  $results,
  type SearchItem,
  blurSearch,
  clearSearch,
  focusSearch,
  searchFx,
  unfocus,
  updateSearch,
} from "./models/model";
import { shardIdAndHeightRegExp, getShardIdAndHeight } from "./utils/shardIdAndHeight";
import { fetchBlockByHash } from "../../api/blocks";

$query.on(updateSearch, (_, query) => query);
$query.reset(clearSearch);

$focused.on(focusSearch, () => true);
$focused.on(blurSearch, () => false);
$focused.on(unfocus, () => false);

searchFx.use(async (query) => {
  const isMessage =
    (query.length === 64 && isHex(query)) ||
    (query.length === 66 && isHex(removeHexPrefix(query)) && query.startsWith("0x"));
  const isAddress =
    (query.length === 40 && isHex(query)) ||
    (query.length === 42 && isHex(removeHexPrefix(query)) && query.startsWith("0x"));

  const isBlockByRegexp = query.match(shardIdAndHeightRegExp);

  if (isMessage) {
    const blockAppend: SearchItem[] = [];
    try {
      const block = await fetchBlockByHash(query);
      blockAppend.push({
        type: "block",
        label: query,
        route: blockRoute,
        params: {
          shard: block.shard_id,
          id: block.id,
        },
      });
      // biome-ignore lint/correctness/noUnusedVariables: <explanation>
    } catch (e) {
      // that's ok
    }
    return [
      ...blockAppend,
      {
        type: "message",
        label: query,
        route: transactionRoute,
        params: {
          hash: removeHexPrefix(query),
        },
      },
    ];
  }

  if (isAddress) {
    return [
      {
        type: "address",
        label: query,
        route: addressRoute,
        params: {
          address: removeHexPrefix(query),
        },
      },
    ];
  }

  if (isBlockByRegexp) {
    const { shardId: shard, height: id } = getShardIdAndHeight(query);
    return [
      {
        type: "block",
        label: query,
        route: blockRoute,
        params: {
          shard,
          id,
        },
      },
    ];
  }

  return [];
});

sample({
  source: $query,
  target: searchFx,
});

$results.on(searchFx.doneData, (_, results) => results);
$results.reset(clearSearch);
