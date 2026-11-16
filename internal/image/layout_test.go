package image

import (
	"strings"
	"testing"
)

func TestCheckGoldenLayoutRendersEverySample(t *testing.T) {
	checks, err := CheckGoldenLayout()
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) == 0 {
		t.Fatal("no golden samples were checked")
	}

	for _, check := range checks {
		if check.Width != check.WantWidth || check.Height != check.WantHeight {
			t.Errorf("%s: dimensions %dx%d, golden %dx%d", check.Name, check.Width, check.Height, check.WantWidth, check.WantHeight)
		}
		if check.Differing != 0 {
			t.Errorf("%s: %d pixels differ from the golden image", check.Name, check.Differing)
		}
	}
}

func TestFormatGroupLessonHandlesBothLessonShapes(t *testing.T) {
	t.Run("single lesson", func(t *testing.T) {
		got := FormatGroupLesson(map[string]any{
			"lesson": "Математика", "type": "лек", "cabinet": "101", "teacher": "Иванов",
		}, 3)
		want := []string{"3", "Математика", "лек", "101", "Иванов"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	})

	t.Run("subgroup list is transposed into five columns", func(t *testing.T) {
		got := FormatGroupLesson([]any{
			map[string]any{"lesson": "Физика", "subgroup": float64(1)},
			map[string]any{"lesson": "Химия", "subgroup": float64(2), "cabinet": "202"},
		}, 1)
		if len(got) != 5 {
			t.Fatalf("%d columns, want five: %q", len(got), got)
		}
		if got[0] != "1"+string(rune(10))+"1" {
			t.Fatalf("the index column must repeat per subgroup: %q", got[0])
		}
		if !strings.Contains(got[1], "Физика") || !strings.Contains(got[1], "Химия") {
			t.Fatalf("the subject column must list both subgroups: %q", got[1])
		}
		if !strings.Contains(got[3], "-") || !strings.Contains(got[3], "202") {
			t.Fatalf("the cabinet column must keep the fallback and the given cabinet: %q", got[3])
		}
		if got[4] != string(rune(10)) {
			t.Fatalf("the teacher column must stay empty: %q", got[4])
		}
	})

	t.Run("nil and foreign values have no lines", func(t *testing.T) {
		if got := FormatGroupLesson(nil, 1); got != nil {
			t.Fatalf("nil lesson = %v", got)
		}
		if got := FormatGroupLesson(42, 1); got != nil {
			t.Fatalf("foreign value = %v", got)
		}
	})
}
