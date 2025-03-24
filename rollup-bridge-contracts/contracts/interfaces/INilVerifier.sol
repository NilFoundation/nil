pragma solidity 0.8.28;

/// @title INilVerifier
/// @notice An interface that lets NilRollup to verify the validityProof.
/// @dev This is the interface used by NilRollup and open for external users to verify the validity proof
interface INilVerifier {
    /// @notice Verify validityProof
    /// @param validityProof The validityProof for state transition
    /// @param publicInput The public input.
    function verify(bytes calldata validityProof, bytes calldata publicInput) external view;
}
