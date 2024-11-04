import { removeHexPrefix, type Hex } from "@nilfoundation/niljs";
import { client } from "../services/clickhouse";
import { z } from "zod";

export const ContractMetadataSchema = z.object({
  source_code: z.record(z.string()),
  abi: z.string(),
});

export const fetchAccountMetadata = async (
  address: Hex,
): Promise<null | z.infer<typeof ContractMetadataSchema>> => {
  const query = await client.query({
    query: `SELECT
        source_code,
        abi
        FROM contracts_metadata
        WHERE
        address = unhex({address: String})
        LIMIT 1`,
    query_params: {
      address: removeHexPrefix(address),
    },
    format: "JSON",
  });

  try {
    const res = (await query.json<z.infer<typeof ContractMetadataSchema>>()).data;
    if (res.length === 0) {
      return null;
    }
    const el = res[0];
    el;
    return el;
  } finally {
    query.close();
  }
};
