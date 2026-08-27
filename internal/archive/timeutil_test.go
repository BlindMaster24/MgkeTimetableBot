package archive

import "testing"

func TestDayIndexToDateRoundtrip(t *testing.T) {
	dates := []string{"01.01.1970", "15.06.2025", "27.08.2026", "31.12.2030"}
	for _, d := range dates {
		idx := DateToDayIndex(d)
		back := DayIndexToDate(idx)
		if back != d {
			t.Errorf("roundtrip failed: %s -> %d -> %s", d, idx, back)
		}
	}
}

func TestDayIndexMonotonic(t *testing.T) {
	d1 := DateToDayIndex("01.01.2025")
	d2 := DateToDayIndex("02.01.2025")
	d3 := DateToDayIndex("03.01.2025")
	if d1 >= d2 || d2 >= d3 {
		t.Errorf("expected monotonic: %d < %d < %d", d1, d2, d3)
	}
	if d2-d1 != 1 || d3-d2 != 1 {
		t.Errorf("expected difference of 1 day")
	}
}

func TestDayIndexInvalid(t *testing.T) {
	idx := DateToDayIndex("invalid")
	if idx != 0 {
		t.Errorf("expected 0 for invalid, got %d", idx)
	}
}
