import type { BitmapCoordinatesRenderingScope, CanvasRenderingTarget2D } from "fancy-canvas";
import type {
  ICustomSeriesPaneRenderer,
  PaneRendererCustomData,
  PriceToCoordinateConverter,
  Time,
} from "lightweight-charts";
import type { GroupedBarsData } from "./GroupedBarsData";
import type { Colors, GroupedBarsSeriesOptions } from "./options";

export interface BitmapPositionLength {
  /** coordinate for use with a bitmap rendering scope */
  position: number;
  /** length for use with a bitmap rendering scope */
  length: number;
}
function centreOffset(lineBitmapWidth: number): number {
  return Math.floor(lineBitmapWidth * 0.5);
}

export function positionsLine(
  positionMedia: number,
  pixelRatio: number,
  desiredWidthMedia = 1,
  widthIsBitmap?: boolean,
): BitmapPositionLength {
  const scaledPosition = Math.round(pixelRatio * positionMedia);
  const lineBitmapWidth = widthIsBitmap
    ? desiredWidthMedia
    : Math.round(desiredWidthMedia * pixelRatio);
  const offset = centreOffset(lineBitmapWidth);
  const position = scaledPosition - offset;
  return { position, length: lineBitmapWidth };
}

export function positionsBox(
  position1Media: number,
  position2Media: number,
  pixelRatio: number,
): BitmapPositionLength {
  const scaledPosition1 = Math.round(pixelRatio * position1Media);
  const scaledPosition2 = Math.round(pixelRatio * position2Media);
  return {
    position: Math.min(scaledPosition1, scaledPosition2),
    length: Math.abs(scaledPosition2 - scaledPosition1) + 1,
  };
}

interface SingleBar {
  x: number;
  y: number;
  color: Colors[number];
}

interface GroupedBarsBarItem {
  singleBars: SingleBar[];
  singleBarWidth: number;
}

export class GroupedBarsSeriesRenderer<TData extends GroupedBarsData>
  implements ICustomSeriesPaneRenderer
{
  _data: PaneRendererCustomData<Time, TData> | null = null;
  _options: GroupedBarsSeriesOptions | null = null;

  draw(target: CanvasRenderingTarget2D, priceConverter: PriceToCoordinateConverter): void {
    target.useBitmapCoordinateSpace((scope) => this._drawImpl(scope, priceConverter));
  }

  update(data: PaneRendererCustomData<Time, TData>, options: GroupedBarsSeriesOptions): void {
    this._data = data;
    this._options = options;
  }

  _drawImpl(
    renderingScope: BitmapCoordinatesRenderingScope,
    priceToCoordinate: PriceToCoordinateConverter,
  ): void {
    if (
      this._data === null ||
      this._data.bars.length === 0 ||
      this._data.visibleRange === null ||
      this._options === null
    ) {
      return;
    }
    const options = this._options;
    const barWidth = this._data.barSpacing;
    const groups: GroupedBarsBarItem[] = this._data.bars.map((bar) => {
      const count = bar.originalData.customValues?.values.length;
      const singleBarWidth = barWidth / (count + 1);
      const padding = singleBarWidth / 2;
      const startX = padding + bar.x - barWidth / 2 + singleBarWidth / 2;
      return {
        singleBarWidth,
        singleBars: bar.originalData.customValues.values.map((value, index) => ({
          y: priceToCoordinate(value) ?? 0,
          color: options.colors[index % options.colors.length],
          x: startX + index * singleBarWidth,
        })),
      };
    });

    const zeroY = priceToCoordinate(0) ?? 0;
    for (let i = this._data.visibleRange.from; i < this._data.visibleRange.to; i++) {
      const group = groups[i];
      let lastX: number;
      // biome-ignore lint/complexity/noForEach: <explanation>
      group.singleBars.forEach((bar) => {
        const ctx = renderingScope.context;

        const yPos = positionsBox(zeroY, bar.y, renderingScope.verticalPixelRatio);
        const xPos = positionsLine(
          bar.x,
          renderingScope.horizontalPixelRatio,
          group.singleBarWidth,
        );

        const offset = lastX ? xPos.position - lastX : 0;

        const radius = 10;
        const x = xPos.position - offset;
        const y = yPos.position;
        const width = xPos.length + offset;
        const height = yPos.length;

        lastX = xPos.position + xPos.length;

        if (width <= 1 || height <= 1) {
          return;
        }

        ctx.beginPath();
        ctx.moveTo(x, y + height);
        ctx.lineTo(x, y + radius);
        ctx.quadraticCurveTo(x, y, x + radius, y);
        ctx.lineTo(x + width - radius, y);
        ctx.quadraticCurveTo(x + width, y, x + width, y + radius);
        ctx.lineTo(x + width, y + height);
        ctx.lineTo(x, y + height);

        if (typeof bar.color === "function") {
          const pattern = bar.color(ctx, () => {
            this._drawImpl(renderingScope, priceToCoordinate);
          });

          ctx.fillStyle = pattern ?? "transparent";
        } else {
          ctx.fillStyle = bar.color;
        }

        ctx.fill();
      });
    }
  }
}
