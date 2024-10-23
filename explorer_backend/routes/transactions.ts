import { z } from "zod";
import { router, publicProcedure } from "../trpc";
import {
  TransactionElemSchema,
  TransactionFullSchema,
  getChildTransactionsByHash,
  getTransactionByHash,
  getTransactionLogsByHash,
  getTransactionsByAddress,
  getTransactionsByBlock,
  getTransactionsByBlockHash,
} from "../daos/transactions";
import { ethAddressSchema } from "../validations/AddressScheme";
import { TransactionLogSchema } from "../validations/TransactionLog";
import { addHexPrefix, removeHexPrefix } from "@nilfoundation/niljs";

export const transactionsRouter = router({
  transactionsByAddress: publicProcedure
    .output(z.array(TransactionElemSchema))
    .input(
      z.object({
        address: ethAddressSchema,
        offset: z.number(),
        limit: z.number(),
      }),
    )
    .query(async (opts) => {
      return getTransactionsByAddress(
        addHexPrefix(opts.input.address),
        opts.input.offset,
        opts.input.limit,
      );
    }),
  transactionsByBlockHash: publicProcedure
    .input(z.string())
    .output(z.array(TransactionElemSchema))
    .query(async (opts) => {
      return getTransactionsByBlockHash(removeHexPrefix(opts.input));
    }),
  transactionsByBlock: publicProcedure
    .input(
      z.object({
        shard: z.number(),
        seqno: z.number(),
      }),
    )
    .output(z.array(TransactionElemSchema))
    .query(async (opts) => {
      const { shard, seqno } = opts.input;
      return getTransactionsByBlock(shard, seqno);
    }),
  transactionByHash: publicProcedure
    .input(z.string())
    .output(TransactionFullSchema)
    .query(async (opts) => {
      const tx = getTransactionByHash(removeHexPrefix(opts.input));
      if (!tx) {
        throw new Error("Transaction not found");
      }
      return tx;
    }),
  getChildTransactionsByHash: publicProcedure
    .input(z.string())
    .output(z.array(TransactionElemSchema))
    .query(async (opts) => {
      return getChildTransactionsByHash(removeHexPrefix(opts.input));
    }),
  getTransactionLogsByHash: publicProcedure
    .input(z.string())
    .output(z.array(TransactionLogSchema))
    .query(async (opts) => {
      const res = await getTransactionLogsByHash(removeHexPrefix(opts.input));
      console.log(res);
      return res;
    }),
});
