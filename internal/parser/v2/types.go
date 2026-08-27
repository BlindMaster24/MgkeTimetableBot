package v2

import "github.com/PuerkitoBio/goquery"

type DayRange struct {
	Day     string
	Weekday string
	Start   int
	Span    int
}

type GridCell struct {
	Cell *goquery.Selection
	Row  int
	Col  int
}

type TableGrid struct {
	Grid   [][]*GridCell
	Width  int
	Height int
}
