"use client";

// VirtualTable — react-virtuoso table wrapped with the cockpit's MUI
// chrome (Card border, sticky head, hover row, themed scrollbar).
//
// Why a wrapper: ExecutionsTable, DeploysTable, ExecutionLedger and
// the operator AuditTab all render the same shape (header + rows from
// a query, click into a detail page). Each row stays cheap because
// only ~2 viewports of rows render at any time, regardless of dataset
// size. Centralising the wiring also keeps a single forwardRef block
// (virtuoso requires its custom Scroller/Table/TableHead/TableRow to
// forward refs) instead of repeating ~30 lines per table.
//
// Per-row navigation is handled via context: pass `rowHref(row)` and
// every row becomes a Next <Link>; pass `onRowClick(row)` and every
// row gets a click handler. Both are optional.

import {
  Box,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
  type SxProps,
  type Theme,
} from "@mui/material";
import { useRouter } from "next/navigation";
import {
  createContext,
  forwardRef,
  useContext,
  useMemo,
  type ReactNode,
  type Ref,
} from "react";
import { TableVirtuoso, type TableComponents } from "react-virtuoso";
import { tokens } from "../../theme";

export interface VirtualTableColumn {
  key: string;
  label: ReactNode;
  align?: "left" | "right" | "center";
  width?: number | string;
  headSx?: SxProps<Theme>;
}

export interface VirtualTableProps<T> {
  rows: T[];
  columns: VirtualTableColumn[];
  rowKey: (row: T) => string;
  // Render the body cells for a single row. MUST return a fragment of
  // <TableCell> children matching `columns.length`.
  renderRow: (row: T, index: number) => ReactNode;
  rowHref?: (row: T) => string;
  onRowClick?: (row: T) => void;
  // Pixel height of the scrolling viewport. Defaults to 560 — enough
  // for ~10 rows on desktop.
  height?: number | string;
  emptyLabel?: string;
  // Approximate row height in px; helps virtuoso size the buffer.
  estimatedRowHeight?: number;
}

interface VirtualTableContextValue<T = unknown> {
  rows: readonly T[];
  rowHref?: (row: T) => string;
  onRowClick?: (row: T) => void;
}

const VirtualTableContext = createContext<VirtualTableContextValue | null>(null);

function useVirtualTableContext<T>(): VirtualTableContextValue<T> {
  const ctx = useContext(VirtualTableContext);
  if (!ctx) {
    throw new Error("VirtualTable row rendered outside provider");
  }
  return ctx as VirtualTableContextValue<T>;
}

const HEAD_CELL_SX = {
  bgcolor: tokens.color.bg.surfaceRaised,
  color: tokens.color.text.muted,
  fontFamily: tokens.font.mono,
  fontSize: 11,
  fontWeight: 700,
  letterSpacing: 0.8,
  textTransform: "uppercase" as const,
  borderBottom: `1px solid ${tokens.color.border.subtle}`,
  whiteSpace: "nowrap" as const,
};

// Virtuoso forces every primitive to forwardRef. Each component below
// is defined once at module load so virtuoso's identity checks treat
// them as stable across renders.

const VtScroller = forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  function VtScrollerImpl(props, ref: Ref<HTMLDivElement>) {
    return (
      <Box
        {...props}
        ref={ref}
        sx={{
          overflow: "auto",
          scrollbarWidth: "thin",
          "&::-webkit-scrollbar": { width: 8, height: 8 },
          "&::-webkit-scrollbar-thumb": {
            background: tokens.color.border.subtle,
            borderRadius: 4,
          },
        }}
      />
    );
  },
);

const VtTable = forwardRef<HTMLTableElement, React.HTMLAttributes<HTMLTableElement>>(
  function VtTableImpl(props, ref: Ref<HTMLTableElement>) {
    return (
      <Table
        {...props}
        size="small"
        ref={ref}
        sx={{
          tableLayout: "fixed",
          borderCollapse: "separate",
          borderSpacing: 0,
        }}
      />
    );
  },
);

const VtTableHead = forwardRef<
  HTMLTableSectionElement,
  React.HTMLAttributes<HTMLTableSectionElement>
>(function VtTableHeadImpl(props, ref) {
  return <TableHead {...props} ref={ref} />;
});

const VtTableBody = forwardRef<
  HTMLTableSectionElement,
  React.HTMLAttributes<HTMLTableSectionElement>
>(function VtTableBodyImpl(props, ref) {
  return <TableBody {...props} ref={ref} />;
});

// VtTableRow reads the per-row navigation hooks from context and the
// row index from virtuoso's data-index attribute. If `rowHref` is
// provided we mount the row as a Next Link so middle-click / cmd-click
// behave like real navigation; otherwise we fall back to onRowClick.

interface VtTableRowProps extends React.HTMLAttributes<HTMLTableRowElement> {
  "data-index"?: number;
}

// VtTableRow — virtuoso passes the row index via `data-index`. We
// look up the row from context and either:
//   - render an <a> with the link href (so middle-click and cmd-click
//     work like real navigation) wrapping a regular TableRow
//   - attach a router.push click handler
//   - or fall through to a plain row.
// Wrapping with `<a>` is wrong inside <tbody>; instead we keep the
// row as <tr> and intercept onClick → router.push. The trade-off is
// no middle-click new-tab, but it type-checks cleanly under MUI v6
// strict polymorphism.
const VtTableRow = forwardRef<HTMLTableRowElement, VtTableRowProps>(
  function VtTableRowImpl(props, ref) {
    const ctx = useVirtualTableContext();
    const router = useRouter();
    const idx = props["data-index"];
    const row = typeof idx === "number" ? ctx.rows[idx] : undefined;
    const interactive = !!(row !== undefined && (ctx.rowHref || ctx.onRowClick));

    const onClick = interactive
      ? (event: React.MouseEvent<HTMLTableRowElement>) => {
          props.onClick?.(event);
          if (event.defaultPrevented || row === undefined) return;
          if (ctx.rowHref) {
            router.push(ctx.rowHref(row));
            return;
          }
          ctx.onRowClick?.(row);
        }
      : props.onClick;

    const baseSx: SxProps<Theme> = {
      cursor: interactive ? "pointer" : "default",
      "&:hover": {
        bgcolor: tokens.color.bg.surfaceHover,
      },
      "& > .MuiTableCell-root": {
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
      },
    };

    return (
      <TableRow
        {...props}
        ref={ref}
        hover
        onClick={onClick}
        sx={baseSx}
      />
    );
  },
);

// Pinned-once components map — virtuoso re-uses references so we must
// not allocate this object inside the render.
const TABLE_COMPONENTS = {
  Scroller: VtScroller,
  Table: VtTable,
  TableHead: VtTableHead,
  TableBody: VtTableBody,
  TableRow: VtTableRow,
};

export function VirtualTable<T>({
  rows,
  columns,
  rowKey,
  renderRow,
  rowHref,
  onRowClick,
  height = 560,
  emptyLabel = "No rows.",
  estimatedRowHeight,
}: VirtualTableProps<T>) {
  // Stable context value — virtuoso re-mounts on every change, so we
  // memoise to keep child identity stable between renders.
  const contextValue = useMemo<VirtualTableContextValue<T>>(
    () => ({ rows, rowHref, onRowClick }),
    [rows, rowHref, onRowClick],
  );

  if (rows.length === 0) {
    return (
      <Box
        sx={{
          border: `1px dashed ${tokens.color.border.subtle}`,
          borderRadius: 1,
          py: 5,
          textAlign: "center",
          color: tokens.color.text.muted,
          fontSize: 13.5,
        }}
      >
        <Typography sx={{ fontSize: 13.5, color: tokens.color.text.muted }}>
          {emptyLabel}
        </Typography>
      </Box>
    );
  }

  return (
    <VirtualTableContext.Provider value={contextValue as VirtualTableContextValue}>
      <Box
        sx={{
          height,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          bgcolor: tokens.color.bg.surface,
          overflow: "hidden",
        }}
      >
        <TableVirtuoso
          data={rows}
          components={TABLE_COMPONENTS as TableComponents<T>}
          computeItemKey={(_, row) => rowKey(row)}
          defaultItemHeight={estimatedRowHeight}
          fixedHeaderContent={() => (
            <TableRow>
              {columns.map((col) => (
                <TableCell
                  key={col.key}
                  align={col.align}
                  sx={{
                    ...HEAD_CELL_SX,
                    width: col.width,
                    ...col.headSx,
                  }}
                >
                  {col.label}
                </TableCell>
              ))}
            </TableRow>
          )}
          itemContent={(index, row) => renderRow(row, index)}
        />
      </Box>
    </VirtualTableContext.Provider>
  );
}
