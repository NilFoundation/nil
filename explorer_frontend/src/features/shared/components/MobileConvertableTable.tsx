import {
  COLORS,
  HeadingXSmall,
  ParagraphSmall,
  TABLE_SIZE,
  TableSemantic,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { ReactNode } from "react";
import { StyledList } from "..";
import { getTabletStyles } from "../../../styleHelpers";

export type MobileConvertableTable = {
  columns: ReactNode[];
  data: ReactNode[][];
  isMobile: boolean;
};

export const MobileConvertableTable = ({ columns, data, isMobile }: MobileConvertableTable) => {
  const [css] = useStyletron();
  const fontStyle = css({
    fontSize: "12px!important",
  });

  if (isMobile) {
    return (
      <StyledList scrollable>
        {data.map((x, index) => {
          return (
            // biome-ignore lint/suspicious/noArrayIndexKey: <explanation>
            <StyledList.Item key={index}>
              {columns.map((column, i) => {
                return (
                  <>
                    <HeadingXSmall
                      color={COLORS.gray400}
                      className={fontStyle}
                      key={`col-1+${
                        // biome-ignore lint/suspicious/noArrayIndexKey: <explanation>
                        i
                      }`}
                    >
                      {column}
                    </HeadingXSmall>
                    <ParagraphSmall
                      key={`col-2+${
                        // biome-ignore lint/suspicious/noArrayIndexKey: <explanation>
                        i
                      }`}
                    >
                      {x[i]}
                    </ParagraphSmall>
                  </>
                );
              })}
            </StyledList.Item>
          );
        })}
      </StyledList>
    );
  }

  return (
    <div
      className={css({
        flexGrow: 1,
      })}
    >
      <div
        className={css({
          ...getTabletStyles({
            minWidth: "990px",
            maxWidth: "1000px",
            width: "100%",
          }),
        })}
      >
        <TableSemantic
          size={TABLE_SIZE.compact}
          horizontalScrollWidth="100%"
          columns={columns}
          data={data}
          overrides={{
            Root: {
              style: () => ({
                backgroundColor: COLORS.gray900,
                maxWidth: "100%",
                overflow: "visible",
              }),
            },
            Table: {
              style: () => ({
                tableLayout: "fixed",
                width: "100%",
                overFlowx: "auto",
              }),
            },
          }}
        />
      </div>
    </div>
  );
};
