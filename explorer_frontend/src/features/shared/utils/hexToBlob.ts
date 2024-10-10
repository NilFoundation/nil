export const hexToBlob = (hex: string) => {
  const bytes = new Uint8Array(hex.match(/[\da-f]{2}/gi)?.map((h) => Number.parseInt(h, 16)));
  return new Blob([bytes], { type: "application/octet-stream" });
};
