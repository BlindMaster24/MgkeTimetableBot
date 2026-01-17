export type DayRange = {
    day: string,
    weekday?: string,
    start: number,
    span: number
}

export type GridCell = {
    cell: HTMLTableCellElement,
    row: number,
    col: number
}

export type TableGrid = {
    grid: GridCell[][],
    width: number,
    height: number
}
