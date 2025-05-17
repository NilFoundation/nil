import axios from "axios";

interface RelayerStats {
    l2SmartAccountAddr: string; // Address of the smart account used by relayer to operate on L2
    timestamp: string; // ISO string for the timestamp
    pendingL1EventCount: number; // Fetched from L1 and not finalized events
    pendingL2EventCount: number; // Finalized events waiting to be sent to L2
    lastProcessedBlock?: number; // Last processed block on L1 (optional)
    currentFinalizedBlock?: number; // Current finalized block on L1 (optional)
}

export async function fetchRelayer(): Promise<string> {
    const relayerRpcUrl = process.env.RELAYER_RPC_URL as string;
    const requestData = {
        jsonrpc: "2.0",
        method: "relayerDebug_getStats",
        params: [],
        id: 1,
    };

    try {
        const response = await axios.post(relayerRpcUrl, requestData, {
            headers: { "Content-Type": "application/json" },
        });

        if (response.data && response.data.result) {
            const stats: RelayerStats = response.data.result;
            console.log("Relayer Stats:", stats);
            return stats.l2SmartAccountAddr;

        } else {
            throw new Error(`Unexpected response: ${JSON.stringify(response.data)}`);
        }
    } catch (error) {
        console.error("Error fetching relayer stats:", error);
        throw error;
    }
}
