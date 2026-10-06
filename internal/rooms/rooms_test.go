package rooms

import (
	"reflect"
	"testing"
)

func TestKeyNormalisesRoomDecorations(t *testing.T) {
	cases := map[string]string{
		"101":           "101",
		" 101 ":         "101",
		"3-212 (к)":     "3-212",
		"3-212(к)":      "3-212",
		"каб. 101":      "101",
		"Каб 2-206":     "2-206",
		"101 (к)":       "101",
		"б":             "б",
		"3-111 (к) ":    "3-111",
		"1-105 (спорт)": "1-105",
		"":              "",
		"   ":           "",
	}
	for room, want := range cases {
		if got := Key(room); got != want {
			t.Errorf("Key(%q) = %q, want %q", room, got, want)
		}
	}
}

func TestKeyKeepsLettersThatBelongToTheNumber(t *testing.T) {
	if Key("101а") == Key("101") {
		t.Fatal("a room with a letter suffix must stay distinct from the plain number")
	}
	if Key("3-212 к") == Key("3-212") {
		t.Fatal("a whitespace suffix is part of the room name, not a decoration")
	}
}

func TestUniverseKeepsOneLabelPerRoom(t *testing.T) {
	universe := Universe([]string{"101", "101 (к)", "101", "202", "  ", "202 (к)"})
	if len(universe) != 2 {
		t.Fatalf("universe = %v, want two rooms", universe)
	}
	if universe["101"] != "101" {
		t.Errorf("label = %q, want the compact one", universe["101"])
	}
	if universe["202"] != "202" {
		t.Errorf("label = %q, want the compact one", universe["202"])
	}
}

func TestUniverseIsIndependentOfTheInputOrder(t *testing.T) {
	first := Universe([]string{"101 (к)", "101", "каб. 101"})
	second := Universe([]string{"каб. 101", "101", "101 (к)"})
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("first = %v, second = %v, want the same labels", first, second)
	}
}

func TestUniverseDropsLabelsThatNormaliseToNothing(t *testing.T) {
	universe := Universe([]string{"(к)", "каб.", "101"})
	if len(universe) != 1 || universe["101"] != "101" {
		t.Fatalf("universe = %v, want only room 101", universe)
	}
}

func TestUniverseBreaksALengthTieAlphabetically(t *testing.T) {
	universe := Universe([]string{"каб.101", "101 (к)"})
	if len(universe) != 1 {
		t.Fatalf("universe = %v, want a single room", universe)
	}
	if universe["101"] != "101 (к)" {
		t.Fatalf("label = %q, want the alphabetically first candidate", universe["101"])
	}
}

func TestUniverseDropsBlankRooms(t *testing.T) {
	universe := Universe([]string{"", "   ", "\t"})
	if len(universe) != 0 {
		t.Fatalf("universe = %v, want none", universe)
	}
}

func TestFreeLeavesOutOccupiedRooms(t *testing.T) {
	universe := Universe([]string{"101", "202", "3-212 (к)"})
	free := Free(universe, map[string]bool{Key("101"): true, Key("3-212"): true})
	if !reflect.DeepEqual(free, []string{"202"}) {
		t.Fatalf("free = %v, want [202]", free)
	}
}

func TestFreeIsSorted(t *testing.T) {
	universe := Universe([]string{"303", "101", "202", "3-212"})
	free := Free(universe, nil)
	want := []string{"101", "202", "3-212", "303"}
	if !reflect.DeepEqual(free, want) {
		t.Fatalf("free = %v, want %v", free, want)
	}
}

func TestFreeReportsEverythingWhenNothingIsOccupied(t *testing.T) {
	universe := Universe([]string{"101", "202"})
	free := Free(universe, map[string]bool{})
	if len(free) != 2 {
		t.Fatalf("free = %v, want both rooms", free)
	}
}

func TestFreeHandlesAnEmptyUniverse(t *testing.T) {
	if free := Free(nil, map[string]bool{"101": true}); len(free) != 0 {
		t.Fatalf("free = %v, want none", free)
	}
}
