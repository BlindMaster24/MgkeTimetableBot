package image

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
)

func encodeSolidPNG(t *testing.T, width, height int, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}
	buffer := &bytes.Buffer{}
	if err := png.Encode(buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestComparePNGReportsSizeAndPixelProblems(t *testing.T) {
	same := encodeSolidPNG(t, 4, 3, 10)
	if got, err := ComparePNG(same, same, 0); err != nil || got != 0 {
		t.Fatalf("identical images = %d %v, want 0", got, err)
	}

	if got, err := ComparePNG(encodeSolidPNG(t, 4, 3, 10), encodeSolidPNG(t, 5, 3, 10), 0); err != nil || got != -1 {
		t.Fatalf("a size change = %d %v, want -1", got, err)
	}

	if got, err := ComparePNG(same, encodeSolidPNG(t, 4, 3, 200), 0); err != nil || got == 0 {
		t.Fatalf("a different shade = %d %v, want differing pixels", got, err)
	}
	if got, err := ComparePNG(same, encodeSolidPNG(t, 4, 3, 200), 200); err != nil || got != 0 {
		t.Fatalf("a tolerant compare = %d %v, want 0", got, err)
	}

	if _, err := ComparePNG([]byte("not a png"), same, 0); err == nil {
		t.Fatal("a broken golden must be an error")
	}
	if _, err := ComparePNG(same, []byte("not a png"), 0); err == nil {
		t.Fatal("a broken render must be an error")
	}
}

func TestChannelDiffIsAlwaysPositive(t *testing.T) {
	if got := channelDiff(5, 3); got != 2 {
		t.Fatalf("channelDiff(5,3) = %d, want 2", got)
	}
	if got := channelDiff(3, 5); got != 2 {
		t.Fatalf("channelDiff(3,5) = %d, want 2", got)
	}
	if got := channelDiff(7, 7); got != 0 {
		t.Fatalf("channelDiff(7,7) = %d, want 0", got)
	}
}

func TestRendererClockFollowsTheInjectedTime(t *testing.T) {
	r := NewRenderer(t.TempDir())
	if r.clock().IsZero() {
		t.Fatal("without an injection the renderer uses the site clock")
	}

	moment := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return moment }
	if got := r.clock(); !got.Equal(moment) {
		t.Fatalf("clock = %s, want %s", got, moment)
	}
}
