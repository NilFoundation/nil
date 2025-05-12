import { COLORS, encodeInlineSvg } from "@nilfoundation/ui-kit";

export const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="8" height="8">
      <rect width="6" height="6" fill="none"/>
      <rect x="6" y="2" width="2" height="2" fill="${COLORS.gray400}"/>
      <rect x="2" y="6" width="2" height="2" fill="${COLORS.gray400}"/>
    </svg>
  `;

export const svgInlined = encodeInlineSvg(svg);

let svgPattern: CanvasPattern | null = null;

const prepareSvgPattern = (ctx: CanvasRenderingContext2D) => {
  const size = 12;
  const svgBlob = new Blob([svg], { type: "image/svg+xml" });
  const url = URL.createObjectURL(svgBlob);
  const img = new Image();

  img.onload = () => {
    const canvas = document.createElement("canvas");
    canvas.width = size;
    canvas.height = size;
    const ctx2d = canvas.getContext("2d");
    if (ctx2d) {
      ctx2d.drawImage(img, 0, 0, size, size);
      svgPattern = ctx.createPattern(canvas, "repeat");
    }
    URL.revokeObjectURL(url);
  };

  img.src = url;
}

export const getSvgPattern = (ctx: CanvasRenderingContext2D) => {
  if (!svgPattern) {
    prepareSvgPattern(ctx);
  }
  return svgPattern;
};
