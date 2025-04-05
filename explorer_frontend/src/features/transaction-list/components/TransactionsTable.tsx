import { COLORS } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type React from "react";
import type { StyleObject } from "styletron-standard";

interface MappedTransactions {
  hash: JSX.Element;
  method: string;
  shardBlock: JSX.Element;
  from: JSX.Element;
  arrow: JSX.Element;
  to: JSX.Element;
  value: string;
  fee: string;
}

interface TransactionsTableProps {
  columns: string[];
  data: MappedTransactions[];
  view: "incoming" | "outgoing" | "all";
}

interface TableRowProps {
  rowData: MappedTransactions;
  view: TransactionsTableProps["view"];
}

const tableWrapperStyles: StyleObject = {
  overflowX: "auto",
  width: "100%",
};

const tableStyles: StyleObject = {
  width: "100%",
  borderCollapse: "collapse",
  color: COLORS.gray50,
  fontSize: "14px",
};

const thStyles: StyleObject = {
  padding: ".75rem 1rem",
  textAlign: "left" as const,
  borderBottom: "1px solid #2F2F2F",
  whiteSpace: "nowrap" as const,
  fontWeight: 500,
};

const tdBaseStyles: StyleObject = {
  padding: ".75rem 1rem",
  whiteSpace: "nowrap" as const,
  verticalAlign: "middle",
};

const tdMaxWidthStyles: StyleObject = {
  ...tdBaseStyles,
  maxWidth: "12.5rem",
};

const methodStyles: StyleObject = {
  paddingInline: ".75rem",
  paddingBlock: ".12rem",
  background: "#2F2F2F",
  display: "grid",
  placeItems: "center",
  borderRadius: "1rem",
};

const TableRow: React.FC<TableRowProps> = ({ rowData, view }) => {
  const [css] = useStyletron();
  const { hash, method, shardBlock, from, arrow, to, value, fee } = rowData;

  const tdStyles = css(tdBaseStyles);
  const tdMaxWStyles = css(tdMaxWidthStyles);

  return (
    <tr>
      <td className={tdMaxWStyles}>{hash}</td>
      <td className={tdStyles}>
        <div className={css(methodStyles)}>{method}</div>
      </td>
      <td className={tdStyles}>{shardBlock}</td>
      <td className={tdMaxWStyles}>{from}</td>
      <td className={tdStyles}>{arrow}</td>
      <td className={tdMaxWStyles}>{to}</td>
      <td className={tdStyles}>{value}</td>
      {view !== "outgoing" && <td className={tdStyles}>{fee}</td>}
    </tr>
  );
};

export const TransactionsTable: React.FC<TransactionsTableProps> = ({ columns, data, view }) => {
  const [css] = useStyletron();

  return (
    <div className={css(tableWrapperStyles)}>
      <table className={css(tableStyles)}>
        <thead>
          <tr>
            {columns.map((column, index) => (
              <th key={`${index}-${column}`} className={css(thStyles)}>
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row) => (
            <TableRow key={`${row}-rowIndex`} rowData={row} view={view} />
          ))}
        </tbody>
      </table>
    </div>
  );
};
