/**
 * Processes an array of transaction receipts and returns the first encountered error message (if any).
 *
 * @param {Array} receipts - An array of transaction receipt objects.
 * @returns {string|null} - Returns an error message if a transaction failed, otherwise returns null.
 */

export function processReceipts(receipts) {
  for (let i = 0; i < receipts.length; i++) {
    const receipt = receipts[i];

    // Check if the transaction was unsuccessful
    if (!receipt.success || receipt.status !== "Success") {
      const errorMessage = receipt.errorMessage || receipt.status; // Use the error message if available, otherwise return the status
      return errorMessage; // Stop processing and return the first encountered error
    }
  }

  // If all transactions are successful, return null indicating no errors
  return null;
}
