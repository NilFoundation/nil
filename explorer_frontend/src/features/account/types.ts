import type { CometaClient } from "@nilfoundation/niljs";
import type { fetchAccountState } from "../../api/account";

export type AccountState = Awaited<ReturnType<typeof fetchAccountState>>;

export type AccountCometaInfo = Awaited<ReturnType<CometaClient['getContract']>
