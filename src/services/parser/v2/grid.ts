import { DayRange, GridCell, TableGrid } from "./types";
import { parseDayLabel } from "./text";

export function buildTableGrid(table: HTMLTableElement): TableGrid {
    const grid: GridCell[][] = [];
    const pending: { cell: HTMLTableCellElement, row: number, col: number, remaining: number }[] = [];

    const rows = Array.from(table.rows);
    for (let rowIndex = 0; rowIndex < rows.length; rowIndex++) {
        const row = rows[rowIndex];
        const current: GridCell[] = [];

        let colIndex = 0;

        while (pending[colIndex] && pending[colIndex].remaining > 0) {
            const active = pending[colIndex];
            current[colIndex] = { cell: active.cell, row: active.row, col: active.col };
            active.remaining -= 1;
            if (active.remaining === 0) {
                delete pending[colIndex];
            }
            colIndex += 1;
        }

        for (const cell of Array.from(row.cells)) {
            while (current[colIndex]) {
                colIndex += 1;
            }

            const colspan = Math.max(1, cell.colSpan || 1);
            const rowspan = Math.max(1, cell.rowSpan || 1);

            for (let offset = 0; offset < colspan; offset++) {
                current[colIndex + offset] = { cell, row: rowIndex, col: colIndex };

                if (rowspan > 1) {
                    pending[colIndex + offset] = {
                        cell,
                        row: rowIndex,
                        col: colIndex,
                        remaining: rowspan - 1
                    };
                }
            }

            colIndex += colspan;
        }

        grid.push(current);
    }

    const width = grid.reduce((max, row) => Math.max(max, row.length), 0);

    return {
        grid,
        width,
        height: grid.length
    };
}

export function findHeaderRowIndex(grid: GridCell[][], maxScan: number = 5, minScore: number = 5): number | null {
    let bestIndex: number | null = null;
    let bestScore = 0;

    const limit = Math.min(maxScan, grid.length);
    for (let i = 0; i < limit; i++) {
        const row = grid[i];
        let score = 0;
        let lastDay: string | null = null;

        for (const cell of row) {
            const parsed = parseDayLabel(cell.cell.textContent);
            if (!parsed) {
                continue;
            }
            if (parsed.day !== lastDay) {
                score += 1;
                lastDay = parsed.day;
            }
        }

        if (score > bestScore) {
            bestScore = score;
            bestIndex = i;
        }
    }

    if (bestScore < minScore) {
        return null;
    }

    return bestIndex;
}

export function getDayRangesFromGrid(grid: GridCell[][], headerRowIndex: number): DayRange[] {
    const headerRow = grid[headerRowIndex];
    if (!headerRow) {
        return [];
    }

    const ranges: DayRange[] = [];
    let colIndex = 0;

    while (colIndex < headerRow.length) {
        const cellRef = headerRow[colIndex];
        if (!cellRef) {
            colIndex += 1;
            continue;
        }

        const parsed = parseDayLabel(cellRef.cell.textContent);
        if (!parsed) {
            colIndex += 1;
            continue;
        }

        let span = 1;
        while (colIndex + span < headerRow.length && headerRow[colIndex + span]?.cell === cellRef.cell) {
            span += 1;
        }

        while (colIndex + span < headerRow.length) {
            const nextCell = headerRow[colIndex + span];
            if (!nextCell) {
                span += 1;
                continue;
            }

            const nextParsed = parseDayLabel(nextCell.cell.textContent);
            if (nextParsed) {
                break;
            }

            span += 1;
        }

        ranges.push({
            day: parsed.day,
            weekday: parsed.weekday,
            start: colIndex,
            span
        });

        colIndex += span;
    }

    return ranges;
}
