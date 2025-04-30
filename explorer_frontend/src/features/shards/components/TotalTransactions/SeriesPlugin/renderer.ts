import type { GroupedBarsData } from "./GroupedBarsData";
import type { GroupedBarsSeriesOptions } from "./options";
import type {
  BitmapCoordinatesRenderingScope,
  CanvasRenderingTarget2D,
} from "fancy-canvas";
import type {
  ICustomSeriesPaneRenderer,
  PaneRendererCustomData,
  PriceToCoordinateConverter,
  Time,
} from "lightweight-charts";

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
  widthIsBitmap?: boolean
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
  pixelRatio: number
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
  color: string;
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

  draw(
    target: CanvasRenderingTarget2D,
    priceConverter: PriceToCoordinateConverter
  ): void {
    target.useBitmapCoordinateSpace(scope => this._drawImpl(scope, priceConverter));
  }

  update(
    data: PaneRendererCustomData<Time, TData>,
    options: GroupedBarsSeriesOptions
  ): void {
    this._data = data;
    this._options = options;
  }

  _drawImpl(
    renderingScope: BitmapCoordinatesRenderingScope,
    priceToCoordinate: PriceToCoordinateConverter
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
    const groups: GroupedBarsBarItem[] = this._data.bars.map(bar => {
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
      group.singleBars.forEach(bar => {
        const yPos = positionsBox(zeroY, bar.y, renderingScope.verticalPixelRatio);
        const xPos = positionsLine(
          bar.x,
          renderingScope.horizontalPixelRatio,
          group.singleBarWidth
        );
        renderingScope.context.beginPath();
        renderingScope.context.fillStyle = bar.color;
        const offset = lastX ? xPos.position - lastX : 0;
        renderingScope.context.fillRect(
          xPos.position - offset,
          yPos.position,
          xPos.length + offset,
          yPos.length
        );
        lastX = xPos.position + xPos.length;
      });
    }
  }
}
