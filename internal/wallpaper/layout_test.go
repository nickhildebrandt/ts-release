package wallpaper

import (
	"strings"
	"testing"

	"golang.org/x/image/font"
)

// mustFacesForHeight loads test font faces whose sizes are scaled relative to the given image height.
// The test fails fast if the embedded fonts cannot be loaded.
func mustFacesForHeight(t *testing.T, height int) (titleFace font.Face, subtitleFace font.Face) {
	t.Helper()

	titleSize := float64(height) * 0.06
	subtitleSize := float64(height) * 0.036

	bold, err := loadFace(boldFontData, titleSize)
	if err != nil {
		t.Fatalf("load bold face: %v", err)
	}
	regular, err := loadFace(regularFontData, subtitleSize)
	if err != nil {
		t.Fatalf("load regular face: %v", err)
	}
	return bold, regular
}

// TestComputeLayoutForText_StandardResolution_ExactMath verifies that the layout formulas match expected values for the standard resolution.
// The test fails on any mismatch in padding, box geometry, or text positions.
func TestComputeLayoutForText_StandardResolution_ExactMath(t *testing.T) {
	titleFace, subtitleFace := mustFacesForHeight(t, TargetHeight)

	title := "TSSH " + strings.Repeat("W", 10)
	subtitle := "build " + strings.Repeat("W", 8)

	l, err := ComputeLayoutForText(TargetWidth, TargetHeight, titleFace, subtitleFace, title, subtitle)
	if err != nil {
		t.Fatalf("ComputeLayoutForText returned error: %v", err)
	}

	titleAdvance := font.MeasureString(titleFace, title).Ceil()
	subAdvance := font.MeasureString(subtitleFace, subtitle).Ceil()
	titleMetrics := titleFace.Metrics()
	subMetrics := subtitleFace.Metrics()
	titleHeight := (titleMetrics.Ascent + titleMetrics.Descent).Ceil()
	subtitleHeight := (subMetrics.Ascent + subMetrics.Descent).Ceil()

	padding := maxInt(14, minInt(TargetWidth, TargetHeight)*paddingPercent/100)
	contentWidth := maxInt(titleAdvance, subAdvance)
	defaultBoxWidth := TargetWidth * boxWidthPercent / 100
	boxWidth := maxInt(defaultBoxWidth, contentWidth+padding*2)

	lineThickness := maxInt(2, TargetHeight/lineThicknessDiv)
	gapAfterTitle := maxInt(padding/3, lineThickness)
	gapAfterSeparator := padding / 2

	boxHeight := padding + titleHeight + gapAfterTitle + lineThickness + gapAfterSeparator + subtitleHeight + padding
	boxX0 := (TargetWidth - boxWidth) / 2
	boxY0 := (TargetHeight - boxHeight) / 2
	boxX1 := boxX0 + boxWidth
	boxY1 := boxY0 + boxHeight

	radius := maxInt(10, minInt(boxWidth, boxHeight)/radiusDivisor)
	titleX := boxX0 + (boxWidth-titleAdvance)/2
	titleY := boxY0 + padding + titleMetrics.Ascent.Ceil()
	separatorY := boxY0 + padding + titleHeight + gapAfterTitle + lineThickness/2
	subtitleX := boxX0 + (boxWidth-subAdvance)/2
	subtitleY := separatorY + lineThickness/2 + gapAfterSeparator + subMetrics.Ascent.Ceil()

	if l.Width != TargetWidth || l.Height != TargetHeight {
		t.Fatalf("unexpected output size %dx%d", l.Width, l.Height)
	}
	if l.Padding != padding {
		t.Fatalf("Padding: got %d want %d", l.Padding, padding)
	}
	if l.BoxWidth != boxWidth || l.BoxHeight != boxHeight {
		t.Fatalf("Box WxH: got %dx%d want %dx%d", l.BoxWidth, l.BoxHeight, boxWidth, boxHeight)
	}
	if l.BoxX0 != boxX0 || l.BoxY0 != boxY0 || l.BoxX1 != boxX1 || l.BoxY1 != boxY1 {
		t.Fatalf("Box corners: got (%d,%d)-(%d,%d) want (%d,%d)-(%d,%d)", l.BoxX0, l.BoxY0, l.BoxX1, l.BoxY1, boxX0, boxY0, boxX1, boxY1)
	}
	if l.BoxRadius != radius {
		t.Fatalf("BoxRadius: got %d want %d", l.BoxRadius, radius)
	}
	if l.SeparatorThickness != lineThickness {
		t.Fatalf("SeparatorThickness: got %d want %d", l.SeparatorThickness, lineThickness)
	}
	if l.SeparatorY != separatorY {
		t.Fatalf("SeparatorY: got %d want %d", l.SeparatorY, separatorY)
	}
	if l.TitleX != titleX || l.TitleY != titleY {
		t.Fatalf("TitleXY: got (%d,%d) want (%d,%d)", l.TitleX, l.TitleY, titleX, titleY)
	}
	if l.SubtitleX != subtitleX || l.SubtitleY != subtitleY {
		t.Fatalf("SubtitleXY: got (%d,%d) want (%d,%d)", l.SubtitleX, l.SubtitleY, subtitleX, subtitleY)
	}

	if l.BoxOpacity != boxOpacityDefault {
		t.Fatalf("BoxOpacity: got %d want %d", l.BoxOpacity, boxOpacityDefault)
	}
	if l.TitleFontSize <= 0 || l.SubtitleFontSize <= 0 {
		t.Fatalf("expected positive font sizes, got title=%v subtitle=%v", l.TitleFontSize, l.SubtitleFontSize)
	}
}

// TestComputeLayoutForText_ScalesWithResolution checks that key layout values scale sensibly with resolution.
// It asserts plausibility bounds for positions, thickness, and radii.
func TestComputeLayoutForText_ScalesWithResolution(t *testing.T) {
	type tc struct{ w, h int }
	cases := []tc{{w: 3840, h: 2160}, {w: 1920, h: 1080}, {w: 640, h: 480}, {w: 7680, h: 4320}}

	titleBase := "TSSH " + strings.Repeat("W", 8)
	subtitleBase := "build " + strings.Repeat("W", 8)

	for _, c := range cases {
		titleFace, subtitleFace := mustFacesForHeight(t, c.h)
		l, err := ComputeLayoutForText(c.w, c.h, titleFace, subtitleFace, titleBase, subtitleBase)
		if err != nil {
			t.Fatalf("ComputeLayoutForText(%dx%d) error: %v", c.w, c.h, err)
		}
		if l.Width != c.w || l.Height != c.h {
			t.Fatalf("ComputeLayoutForText(%dx%d) returned layout size %dx%d", c.w, c.h, l.Width, l.Height)
		}

		wantPadding := maxInt(14, minInt(c.w, c.h)*paddingPercent/100)
		wantLineThickness := maxInt(2, c.h/lineThicknessDiv)
		if l.Padding != wantPadding {
			t.Fatalf("%dx%d Padding: got %d want %d", c.w, c.h, l.Padding, wantPadding)
		}
		if l.SeparatorThickness != wantLineThickness {
			t.Fatalf("%dx%d SeparatorThickness: got %d want %d", c.w, c.h, l.SeparatorThickness, wantLineThickness)
		}
		if l.BoxWidth <= 0 || l.BoxHeight <= 0 {
			t.Fatalf("%dx%d invalid box WxH: %dx%d", c.w, c.h, l.BoxWidth, l.BoxHeight)
		}
		if l.BoxX1 <= l.BoxX0 || l.BoxY1 <= l.BoxY0 {
			t.Fatalf("%dx%d invalid box corners (%d,%d)-(%d,%d)", c.w, c.h, l.BoxX0, l.BoxY0, l.BoxX1, l.BoxY1)
		}
		if l.BoxRadius <= 0 {
			t.Fatalf("%dx%d expected positive radius, got %d", c.w, c.h, l.BoxRadius)
		}
		if l.TitleX < 0 || l.TitleY < 0 || l.SubtitleX < 0 || l.SubtitleY < 0 {
			t.Fatalf("%dx%d expected non-negative text positions, got title (%d,%d) subtitle (%d,%d)", c.w, c.h, l.TitleX, l.TitleY, l.SubtitleX, l.SubtitleY)
		}
	}
}

// TestComputeLayoutForText_BoxWidthUsesWiderText ensures the box width is based on the wider of title or subtitle.
// The test fails if the computed box would truncate text.
func TestComputeLayoutForText_BoxWidthUsesWiderText(t *testing.T) {
	w, h := 3840, 2160
	titleFace, subtitleFace := mustFacesForHeight(t, h)

	baseTitle := "TSSH"
	baseSubtitle := "build"

	cases := []struct {
		name      string
		title     string
		subtitle  string
		wantWider string
	}{
		{name: "subtitle longer", title: baseTitle, subtitle: baseSubtitle + " " + strings.Repeat("W", 18), wantWider: "subtitle"},
		{name: "title longer", title: baseTitle + " " + strings.Repeat("W", 18), subtitle: baseSubtitle, wantWider: "title"},
		{name: "equal", title: baseTitle + strings.Repeat("W", 8), subtitle: baseSubtitle + strings.Repeat("W", 8), wantWider: "equal"},
	}

	for _, c := range cases {
		l, err := ComputeLayoutForText(w, h, titleFace, subtitleFace, c.title, c.subtitle)
		if err != nil {
			t.Fatalf("%s: error: %v", c.name, err)
		}

		padding := maxInt(14, minInt(w, h)*paddingPercent/100)
		defaultBoxWidth := w * boxWidthPercent / 100
		titleAdvance := font.MeasureString(titleFace, c.title).Ceil()
		subAdvance := font.MeasureString(subtitleFace, c.subtitle).Ceil()
		contentWidth := maxInt(titleAdvance, subAdvance)
		wantBoxWidth := maxInt(defaultBoxWidth, contentWidth+padding*2)

		if l.BoxWidth != wantBoxWidth {
			t.Fatalf("%s: BoxWidth got %d want %d", c.name, l.BoxWidth, wantBoxWidth)
		}

		switch c.wantWider {
		case "subtitle":
			if subAdvance < titleAdvance {
				t.Fatalf("%s: expected subtitle wider; got titleAdvance=%d subAdvance=%d", c.name, titleAdvance, subAdvance)
			}
		case "title":
			if titleAdvance < subAdvance {
				t.Fatalf("%s: expected title wider; got titleAdvance=%d subAdvance=%d", c.name, titleAdvance, subAdvance)
			}
		}
	}
}

// TestComputeLayoutForText_ErrorsOnNilFaces expects an error when no font faces are provided.
// This documents the minimum precondition for layout computation.
func TestComputeLayoutForText_ErrorsOnNilFaces(t *testing.T) {
	_, err := ComputeLayoutForText(3840, 2160, nil, nil, "t", "s")
	if err == nil {
		t.Fatalf("expected error for nil font faces")
	}
	if got := err.Error(); !strings.Contains(got, "font face is nil") {
		t.Fatalf("unexpected error: %q", got)
	}
}
