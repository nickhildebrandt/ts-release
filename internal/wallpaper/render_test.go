package wallpaper

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"golang.org/x/image/font"
)

func solidBG(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func titleAndSubtitleFor(targetName, buildID string) (string, string) {
	title := strings.TrimSpace(targetName)
	if title == "" {
		title = "TSSH"
	} else {
		title = "TSSH " + title
	}
	subtitle := strings.TrimSpace(buildID)
	if subtitle == "" {
		subtitle = "build unknown"
	}
	return title, subtitle
}

func mustRenderFaces(t *testing.T) (font.Face, font.Face) {
	t.Helper()
	titleSize := float64(TargetHeight) * 0.06
	subtitleSize := float64(TargetHeight) * 0.036

	titleFace, err := loadFace(boldFontData, titleSize)
	if err != nil {
		t.Fatalf("load title face: %v", err)
	}
	subtitleFace, err := loadFace(regularFontData, subtitleSize)
	if err != nil {
		t.Fatalf("load subtitle face: %v", err)
	}
	return titleFace, subtitleFace
}

func mustMaxTextWidth(t *testing.T) int {
	t.Helper()
	maxW, err := maxTextWidthForImage(TargetWidth)
	if err != nil {
		t.Fatalf("maxTextWidthForImage error: %v", err)
	}
	return maxW
}

func findLenBoundary(t *testing.T, label string, face font.Face, prefix string, wantLen int, maxWidth int) (ok string, tooLong string) {
	t.Helper()
	if wantLen <= 0 {
		t.Fatalf("invalid wantLen %d", wantLen)
	}

	// Search for a deterministic pair where:
	// - ok has length wantLen and fits.
	// - tooLong has length wantLen+1 (ok + "W") and fails.
	// We vary how many wide characters we include to get near the boundary.
	for wide := wantLen; wide >= 0; wide-- {
		candidate := strings.Repeat("W", wide) + strings.Repeat("i", wantLen-wide)
		if len(candidate) != wantLen {
			continue
		}
		if err := validateTextWidth(label, face, prefix+candidate, maxWidth); err != nil {
			continue
		}
		candidateTooLong := candidate + "W"
		if len(candidateTooLong) != wantLen+1 {
			continue
		}
		if err := validateTextWidth(label, face, prefix+candidateTooLong, maxWidth); err == nil {
			continue
		}
		return candidate, candidateTooLong
	}

	t.Fatalf("failed to find %d/%d length boundary for %s", wantLen, wantLen+1, label)
	return "", ""
}

func findTooLongText(t *testing.T, label string, face font.Face, prefix string, maxWidth int) string {
	t.Helper()
	// Find a deterministic text that fails validateTextWidth by increasing width.
	for n := 1; n <= 512; n++ {
		candidate := strings.Repeat("W", n)
		if err := validateTextWidth(label, face, prefix+candidate, maxWidth); err != nil {
			return candidate
		}
	}
	t.Fatalf("failed to find too-long text for %s", label)
	return ""
}

func TestRender_ReturnsTargetResolution(t *testing.T) {
	bg := solidBG(64, 64, color.RGBA{0, 0, 0, 255})
	img, err := Render(bg, "test", "build-1")
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img == nil {
		t.Fatalf("expected non-nil image")
	}
	b := img.Bounds()
	if b.Dx() != TargetWidth || b.Dy() != TargetHeight {
		t.Fatalf("unexpected size %dx%d", b.Dx(), b.Dy())
	}
}

func TestRender_EmptyTargetNameAndSubtitle_DefaultsAndNoPanic(t *testing.T) {
	bg := solidBG(16, 16, color.RGBA{0, 0, 0, 255})
	img, err := Render(bg, "   ", "")
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img == nil {
		t.Fatalf("expected non-nil image")
	}
}

func TestRender_ErrorsOnNilBackground(t *testing.T) {
	img, err := Render(nil, "x", "y")
	if err == nil {
		t.Fatalf("expected error")
	}
	if img != nil {
		t.Fatalf("expected nil image on error")
	}
	if !strings.Contains(err.Error(), "background is nil") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

func TestRender_TextTooLong_Boundaries_26vs27(t *testing.T) {
	bg := solidBG(32, 32, color.RGBA{0, 0, 0, 255})
	titleFace, subtitleFace := mustRenderFaces(t)
	maxW := mustMaxTextWidth(t)

	okTarget26, tooLongTarget27 := findLenBoundary(t, "title", titleFace, "TSSH ", 26, maxW)
	tooLongSubtitle := findTooLongText(t, "subtitle", subtitleFace, "", maxW)

	cases := []struct {
		name      string
		target    string
		buildID   string
		wantError bool
	}{
		{name: "target empty becomes default", target: "", buildID: "", wantError: false},
		{name: "target len 26 ok", target: okTarget26, buildID: "id", wantError: false},
		{name: "target len 27 too long", target: tooLongTarget27, buildID: "id", wantError: true},
		{name: "subtitle too long", target: "ok", buildID: tooLongSubtitle, wantError: true},
	}

	for _, c := range cases {
		img, err := Render(bg, c.target, c.buildID)
		if c.wantError {
			if err == nil {
				t.Fatalf("%s: expected error", c.name)
			}
			if img != nil {
				t.Fatalf("%s: expected nil image on error", c.name)
			}
			if got := err.Error(); !strings.Contains(got, "too long") {
				t.Fatalf("%s: error does not indicate too long: %q", c.name, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", c.name, err)
		}
		if img == nil {
			t.Fatalf("%s: expected non-nil image", c.name)
		}
	}
}

func TestDrawSeparator_WidthUsesWiderOfTitleOrSubtitle(t *testing.T) {
	bg := solidBG(32, 32, color.RGBA{0, 0, 0, 255})

	titleFace, subtitleFace := mustRenderFaces(t)

	cases := []struct {
		name    string
		target  string
		buildID string
	}{
		{name: "subtitle wider", target: "ok", buildID: "build " + strings.Repeat("W", 10)},
		{name: "title wider", target: strings.Repeat("W", 10), buildID: "b"},
	}

	for _, c := range cases {
		img, err := Render(bg, c.target, c.buildID)
		if err != nil {
			t.Fatalf("%s: Render error: %v", c.name, err)
		}

		title, subtitle := titleAndSubtitleFor(c.target, c.buildID)
		layout, err := ComputeLayoutForText(TargetWidth, TargetHeight, titleFace, subtitleFace, title, subtitle)
		if err != nil {
			t.Fatalf("%s: ComputeLayoutForText error: %v", c.name, err)
		}

		titleW := font.MeasureString(titleFace, title).Ceil()
		subW := font.MeasureString(subtitleFace, subtitle).Ceil()
		textW := maxInt(titleW, subW)
		maxW := layout.BoxWidth - 2*layout.Padding
		if textW > maxW {
			textW = maxW
		}
		extra := maxInt(layout.Padding/4, 10)
		desired := textW + extra
		if desired > maxW {
			desired = maxW
		}

		// Scan the separator row inside the box and find the bright pixels.
		y := layout.SeparatorY
		if y < 0 || y >= img.Bounds().Dy() {
			t.Fatalf("%s: separator Y out of bounds: %d", c.name, y)
		}

		startX := -1
		endX := -1
		for x := layout.BoxX0; x < layout.BoxX1; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// The separator is a bright white-ish line drawn over a dark box.
			if r>>8 > 50 && g>>8 > 50 && b>>8 > 50 {
				if startX == -1 {
					startX = x
				}
				endX = x
			}
		}

		if startX == -1 || endX == -1 {
			t.Fatalf("%s: failed to locate separator pixels on row %d", c.name, y)
		}

		gotWidth := endX - startX + 1
		if gotWidth != desired {
			t.Fatalf("%s: separator width got %d want %d (titleW=%d subW=%d)", c.name, gotWidth, desired, titleW, subW)
		}
	}
}
