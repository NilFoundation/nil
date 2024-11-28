import { useState, useEffect } from "react";
import type { Transaction } from "../../transaction/types/Transaction";
import { fetchTransactionByHash } from "../../../api/transaction";
import type { Hex } from "@nilfoundation/niljs";

export const useRepeatedGetTxByHash = (
  txHash: Hex,
  interval = 1000,
): {
  data: Transaction | null;
  error: boolean;
  loading: boolean;
} => {
  const [data, setData] = useState<Transaction | null>(null);
  const [error, setError] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const controller = new AbortController();
    const signal = controller.signal;

    const fetchData = async () => {
      try {
        const response = await fetchTransactionByHash(txHash, {
          signal,
        });

        if (response) {
          setData(response);
          setError(false);
          setLoading(false);
          clearInterval(intervalId);
        }
      } catch (err) {
        setError(true);
        setLoading(false);
      }
    };

    const intervalId = setInterval(fetchData, interval);

    return () => {
      clearInterval(intervalId);
      controller.abort();
    };
  }, [interval, txHash]);

  return { data, error, loading };
};
