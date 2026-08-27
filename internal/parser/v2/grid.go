package v2

import (
	"math"

	"github.com/PuerkitoBio/goquery"
)

type pendingCell struct {
	cell      *goquery.Selection
	row       int
	col       int
	remaining int
}

func BuildTableGrid(table *goquery.Selection) *TableGrid {
	grid := make([][]*GridCell, 0)
	pending := make(map[int]*pendingCell)

	rows := table.Find("tr")
	rows.Each(func(rowIndex int, row *goquery.Selection) {
		current := make([]*GridCell, 0)

		colIndex := 0
		for {
			if p, ok := pending[colIndex]; ok && p.remaining > 0 {
				current = append(current, &GridCell{Cell: p.cell, Row: p.row, Col: p.col})
				p.remaining--
				if p.remaining == 0 {
					delete(pending, colIndex)
				}
				colIndex++
				continue
			}
			break
		}

		cells := row.Find("td, th")
		cells.Each(func(_ int, cell *goquery.Selection) {
			for len(current) <= colIndex {
				current = append(current, nil)
			}

			colspan := getMax(1, getAttrInt(cell, "colspan"))
			rowspan := getMax(1, getAttrInt(cell, "rowspan"))

			for offset := 0; offset < colspan; offset++ {
				idx := colIndex + offset
				for len(current) <= idx {
					current = append(current, nil)
				}
				current[idx] = &GridCell{Cell: cell, Row: rowIndex, Col: colIndex}

				if rowspan > 1 {
					pending[idx] = &pendingCell{
						cell:      cell,
						row:       rowIndex,
						col:       colIndex,
						remaining: rowspan - 1,
					}
				}
			}

			colIndex += colspan
		})

		grid = append(grid, current)
	})

	width := 0
	for _, row := range grid {
		if len(row) > width {
			width = len(row)
		}
	}

	return &TableGrid{
		Grid:   grid,
		Width:  width,
		Height: len(grid),
	}
}

func FindHeaderRowIndex(grid *TableGrid, maxScan int, minScore int) int {
	bestIndex := -1
	bestScore := 0

	limit := getIntMin(maxScan, grid.Height)
	for i := 0; i < limit; i++ {
		row := grid.Grid[i]
		score := 0
		var lastDay string

		for _, cellRef := range row {
			if cellRef == nil {
				continue
			}
			day, _ := parseDayLabel(cellRef.Cell.Text())
			if day == "" {
				continue
			}
			if day != lastDay {
				score++
				lastDay = day
			}
		}

		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}

	if bestScore < minScore {
		return -1
	}

	return bestIndex
}

func GetDayRangesFromGrid(grid *TableGrid, headerRowIndex int) []DayRange {
	if headerRowIndex < 0 || headerRowIndex >= grid.Height {
		return nil
	}

	headerRow := grid.Grid[headerRowIndex]
	var ranges []DayRange
	colIndex := 0

	for colIndex < len(headerRow) {
		cellRef := headerRow[colIndex]
		if cellRef == nil {
			colIndex++
			continue
		}

		day, weekday := parseDayLabel(cellRef.Cell.Text())
		if day == "" {
			colIndex++
			continue
		}

		span := 1
		for colIndex+span < len(headerRow) && headerRow[colIndex+span] != nil && headerRow[colIndex+span].Cell.Get(0) == cellRef.Cell.Get(0) {
			span++
		}

		for colIndex+span < len(headerRow) {
			nextCell := headerRow[colIndex+span]
			if nextCell == nil {
				span++
				continue
			}
			nextDay, _ := parseDayLabel(nextCell.Cell.Text())
			if nextDay != "" {
				break
			}
			span++
		}

		ranges = append(ranges, DayRange{
			Day:     day,
			Weekday: weekday,
			Start:   colIndex,
			Span:    span,
		})

		colIndex += span
	}

	return ranges
}

func getAttrInt(sel *goquery.Selection, attr string) int {
	val := sel.AttrOr(attr, "0")
	if val == "" || val == "0" {
		return 1
	}
	n := 0
	for _, c := range val {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

func getMax(a, b int) int {
	return int(math.Max(float64(a), float64(b)))
}

func getIntMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
