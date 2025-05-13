TLDR: This post suggests a pathway for EVM-equivalent rollups to rid themselves of security councils and other attack vectors, unlocking full Ethereum L1 security.

Credit for this post goes to the wider Ethereum R&D community. Key contributions originated from 2017, with significant incremental design unlocks over the years. A thorough design space exploration was instigated by recent zkVM engineering breakthroughs. This write-up is merely a best effort attempt to piece together a coherent design for a big idea whose time may have finally come.

Abstract: We propose an elegant and powerful EXECUTE precompile exposing the native L1 EVM execution engine to the application layer. A native execution rollup, or “native rollup” for short, is a rollup which uses EXECUTE to verify EVM state transitions for batches of user transactions. One can think of native rollups as “programmable execution shards” that wrap the precompile within a derivation function to handle extra-EVM system logic, e.g. sequencing, bridging, forced inclusion, governance.

Because the EXECUTE precompile is directly enforced by validators it enjoys (zk)EL client diversity and provides EVM equivalence which is by construction bug-free and forward-compatible with EVM upgrades through L1 hard forks. A form of EVM introspection like the EXECUTE precompile is necessary for EVM-equivalent rollups that wish to fully inherit Ethereum security. We call rollups that fully inherit Ethereum security “trustless rollups”.

The EXECUTE precompile significantly simplifies development of EVM-equivalent rollups by removing the need for complex infrastructure—such as fraud proof games, SNARK circuits, security councils—for EVM emulation and maintenance. With EXECUTE one can deploy minimal native and based rollups in just a few lines of Solidity code with a simple derivation function that obviates the need for special handling of sequencing, forced inclusion, or governance.

As a cherry on top native rollups can enjoy real-time settlement without needing to worry about real-time proving, dramatically simplifying synchronous composability.

This write-up is structured in two parts, starting with a description of the proposed precompile and ending with a discussion of native rollups.

Part 1—the EXECUTE precompile

construction

The EXECUTE precompile takes inputs pre_state_root, post_state_root, trace, and gas_used. It returns true if and only if:

trace is a well-formatted execution trace (e.g. a list of L2 transactions and corresponding state access proofs)
the stateless execution of trace starting from pre_state_root ends at post_state_root
the stateless execution of trace consumes exactly gas_used gas
There is an EIP-1559-style mechanism to meter and price cumulative gas consumed across all EXECUTE calls in an L1 block. In particular, there is a cumulative gas limit EXECUTE_CUMULATIVE_GAS_LIMIT, and a cumulative gas target EXECUTE_CUMULATIVE_GAS_TARGET. (The cumulative limit and target could be merged with the L1 EIP-1559 mechanism when the L1 EVM is statelessly enforceable by validators.)

Calling the precompile costs a fixed amount of L1 gas, EXECUTE_GAS_COST, plus gas_used * gas_price where gas_price (denominated in ETH/gas) is set by the EIP-1559-style mechanism. Full upfront payment is drawn, even when the precompile returns false.

The trace must point to available Ethereum data from calldata, blobs, state, or memory.

enforcement by re-execution

If EXECUTE_CUMULATIVE_GAS_LIMIT is small enough validators can naively re-execute traces to enforce correctness of EXECUTE calls. An initial deployment of the precompile based on re-execution could serve as a stepping stone, similar to what proto-danksharding with naive re-downloading of blobs is to full danksharding. Notice that naive re-execution imposes no state growth or bandwidth overhead for validators, and any execution overhead is parallelisable across CPU cores.

Validators must hold an explicit copy of trace for re-execution, preventing the use of pointers to blob data which are sampled (not downloaded) via DAS. Notice that optimistic native rollups may still post rollup data in blobs, falling back to calldata only in the fraud proof game. Notice also that optimistic native rollups can have a gas limit that far surpasses EXECUTE_CUMULATIVE_GAS_LIMIT because the EXECUTE precompile only needs to be called once on a small EVM segment to settle a fraud proof challenge.

As a historical note, in 2017 Vitalik suggested a similar “EVM inside EVM” precompile called EXECTX.

enforcement by SNARKs

To unlock a large EXECUTE_CUMULATIVE_GAS_LIMIT it is natural to have validators optionally verify SNARK proofs. From now on we assume one-slot delayed execution where invalid blocks (or invalid transactions) are treated as no-ops. (For more information about delayed execution see this ethresearch post, this EIP, and this design by Francesco.) One-slot delayed execution yield several seconds—a whole slot—for proving. They also avoid incentivising MEV-driven proof racing which would introduce a centralisation vector.

Note that even when EXECUTE is enforced by SNARKs no explicit proof system or circuit is enshrined in consensus. (Notice the EXECUTE precompile does not take any explicitly proof as input.) Instead, each staking operator is free to choose their favourite zkEL verifier client(s) similar to how EL clients are subjectively chosen today. The benefits of this design decision are explained in the next section titled “offchain proofs”.

From now on we assume that execution proposers are sophisticated in the context of Attester-Proposer Separation (APS) with alternating execution and consensus slots. To incentivise rational execution proposers to generate proofs in a timely fashion (within 1 slot) we mandate attesters only attest to execution block n+1 if proofs for execution block n are available. (We suggest bundling block n+1 with EXECUTE proofs for block n at the p2p layer.) An execution proposer that skips proving may miss their slot, leading to missed fees and MEV. We further apply a fixed penalty for missed execution slots, setting it high enough (e.g. 1 ETH) to always surpass the cost of proving.

Note that in the context of APS the production of consensus blocks is not blocked by missed execution slots. Timely generation of proofs is however relevant for light clients to easily read state at the chain tip, without stateless re-execution. To ensure proofs are generated in a timely fashion for light clients, even in the exceptional case where the next execution proposer misses their slot, we rely on an altruistic-minority prover assumption. A single altruistic prover is sufficient to generate proofs within 1 slot. To avoid unnecessary redundant proving, most altruistic provers can wait on standby and kick in only when no proof arrives within 1 slot, thereby acting as a fail safe with at most 2-slot latency.

Note that EXECUTE_CUMULATIVE_GAS_LIMIT needs to be set low enough for the altruistic-minority prover assumption to be credible (as well as for execution proposing to not be unrealistically sophisticated). A conservative policy could be to set EXECUTE_CUMULATIVE_GAS_LIMIT so that single-slot proving is accessible to laptops, e.g. high-end MacBook Pros. A more pragmatic and aggressive policy could be to target a small cluster of GPUs, and maybe eventually SNARK ASIC provers once those are sufficiently commoditised.

offchain proofs

To reiterate, we suggest that zkEL EXECUTE proofs not go onchain and instead be shared offchain. Not enshrining proofs is a beautiful idea first suggested by Vitalik which comes with several advantages:

diversity: Validators are free to choose proof verifiers (including proof systems and circuits) from dev teams they trust, similar to how validators choose EL clients they trust. This provides robustness through diversity. zkEL verifier clients (and the zkVMs that would underlie some) are complex pieces of cryptographic software. A bug in any one client should not take down Ethereum.
neutrality: Having a market of zkEL verifier clients allows for the consensus layer to not pick technology winners. For example, the zkVM market is highly competitive and picking a winning vendor such as Risc0, Succinct, or the many other vendors may not be perceived as neutral.
simplicity: The consensus layer need not enshrine a specific SNARK verifier, dramatically simplifying the specification of the consensus layer. It is sufficient to enshrine a format for state access proofs, not specific proof verifier implementation details.
flexibility: Should a bug or optimisation be found, affected validators can update their client without the need for a hard fork.
Having offchain proofs does introduce a couple manageable complications:

prover load and p2p fragmentation: Because there isn’t a single canonical proof, multiple proofs (at least one per zkEL client) needs to be generated. Every zkEL client customisation (e.g. swapping one RISC-V zkVM for another) requires a different proof. Likewise, every zkEL version bump requires a different proof. This will lead to increased proving load. It will additionally fragment the p2p network if there’s a separate gossip channel per proof type.
minority zkELs: It’s hard to incentivise proof generation for minority zkELs. Rational execution proposers may only generate sufficient proofs so as to reach a super-majority of attesters and not miss their slot. To combat this, staking operators could be socially encouraged to run multiple zkEL clients in parallel similar to Vouch operators today. Running a k-of-n setup has the additional benefit of boosting security, in particular hedging against soundness bugs that allow an attacker to craft proofs for arbitrary EXECUTE calls (a situation that would be unusual for a traditional EL client).
Offchain proofs also introduces inefficiencies for real-time settled L2s:

no alt DA: Because the trace input to EXECUTE needs to have been made available to L1 validators, real-time settled L2 (i.e. L2s that immediately update their canonical state root) must consume L1 DA, i.e. be rollups. Notice that optimistic L2s that delay settlement via a fraud proof game do not have this limitation, i.e. can be validiums.
state access overhead: Because the trace must be statelessly executable it must include state trie leaves that are read or written, introducing a small DA overhead over a typical L2 block. Notice that optimistic L2s do not have this limitation because state trie leaves are only required in fraud proof challenges and the challenger can recompute trie leaves.
no state diffing: Because proving should be permissionless given the trace, rollup state diffing is not possible. It would however be possible to compress stateless access proofs or EVM transaction signatures if corresponding specialised proofs were enshrined in consensus.
RISC-V native execution

Given today’s de facto convergence towards RISC-V zkVMs there may be an opportunity to expose RISC-V state transitions natively to the EVM (similarly to WASM in the context of Arbitrum Stylus) and remain SNARK-friendly.

Part 2—native rollups

naming

We start by discussing the naming of native rollups to address several sources of confusion:

alternative names: Native rollups were previously referred to as enshrined rollups, see for example this writeup and this writeup. (The term “canonical rollup” was also briefly used by Polynya.) The term “enshrined” was later abandoned in favour of “native” to signal that existing that EVM-equivalent rollups have the option to upgrade to become native. The name “native” was independently suggested in November 2022 by Dan Robinson and a Lido contributor which wishes to remain anonymous.
based rollups: Based rollups and native rollups are orthogonal concepts: “based” relates to L1 sequencing whereas “native” relates to L1 execution. A rollup that is simultaneously based and native is whimsically called an “ultra sound rollup”.
execution shards: Execution shards (i.e. enshrined copies of L1 EVM chains) is a different but related concept related to native rollups, predating native rollups by several years. (Execution sharding was previously “phase 2” of the Ethereum 2.0 roadmap.) Unlike native rollups, execution shards are non-programmable, i.e. without the option for custom governance, custom sequencing, custom gas token, etc. Execution shards are also typically instantiated in a fixed quantity (e.g. 64 or 1,024 shards). Unfortunately Martin Köppelmann used the term “native L2” in his 2024 Devcon talk about execution shards.
benefits

Native rollups have several benefits which we detail below:

simplicity: Most of the sophistication of a native rollup VM can be encapsulated by the precompile. Today’s EVM-equivalent optimistic and zk-rollups have thousands of lines of code for their fraud proof game or SNARK verifier that could collapse to a single line of code. Native rollups also don’t require ancillary infrastructure like proving networks, watchtowers, and security councils.
security: Building a bug-free EVM fraud proof game or SNARK verifier is a remarkably difficult engineering task that likely requires deep formal verification. Every optimistic and zk EVM rollup most likely has critical vulnerabilities today in their EVM state transition function. To defend against vulnerabilities, centralised sequencing is often used as a crutch to gate the production of adversarially-crafted blocks. The native execution precompile allows for the safe deployment of permissionless sequencing. Trustless rollups that fully inherit L1 security additionally fully inherit L1 asset fungibility.
EVM equivalence: Today the only way for a rollup to remain in sync with L1 EVM rules is to have governance (typically a security council and/or a governance token) to mirror L1 EVM upgrades. (EVM updates still happen regularly via hard forks roughly once per year.) Not only is governance an attack vector, it is strictly-speaking a departure from the L1 EVM and prevents any rollup from achieving true long-term EVM equivalence. Native rollups on the other hand can upgrade in unison with the L1, governance-free.
SNARK gas cost: Verifying SNARKs onchain is expensive. As a result many zk-rollups settle infrequently to minimise costs. Since SNARKs are not verified onchain the EXECUTE precompile could be used as a way to lower the cost of verification. If EXECUTE proofs across multiple calls in a block are batched using SNARK recursion EXECUTE_GAS_COST could be set relatively low.
synchronous composability: Today synchronous composability with the L1 requires same-slot real-time proving. For zk rollups achieving ultra-low-latency proving, e.g. on the order of 100ms, is an especially challenging engineering task. With one-slot delayed state root the proving latency underlying the native execution precompile can be relaxed to one full slot.