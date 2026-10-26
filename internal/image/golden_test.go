package image

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGoldenImages = flag.Bool("update", false, "rewrite the golden timetable images")

func TestGoldenLayoutMatches(t *testing.T) {
	rendered, err := RenderGoldenSamples()
	if err != nil {
		t.Fatalf("render the golden samples: %v", err)
	}

	if *updateGoldenImages {
		if err := os.MkdirAll("golden", 0o755); err != nil {
			t.Fatalf("create the golden directory: %v", err)
		}
		for name, data := range rendered {
			if err := os.WriteFile(filepath.Join("golden", name+".png"), data, 0o644); err != nil {
				t.Fatalf("write the %s golden: %v", name, err)
			}
		}
		return
	}

	for name, want := range GoldenImages() {
		got, ok := rendered[name]
		if !ok {
			t.Fatalf("the renderer produced no %s sample", name)
		}

		differing, err := ComparePNG(want, got, 8)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if differing < 0 {
			keepReceivedImage(t, name, got)
			t.Fatalf("the %s sample changed size; the layout moved", name)
		}
		if differing > 0 {
			path := keepReceivedImage(t, name, got)
			t.Fatalf("%d pixels of the %s sample differ from golden/%s.png (received image kept in %s): review the render and run go test ./internal/image -update", differing, name, name, path)
		}
	}
}

func keepReceivedImage(t *testing.T, name string, data []byte) string {
	t.Helper()

	path := filepath.Join("golden", name+".png.got.png")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("keep the received %s image: %v", name, err)
	}
	return path
}
