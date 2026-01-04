package wallpaper

import (
	"fmt"

	"golang.org/x/image/font"
)

// Layout defines all geometry and font sizing used during rendering.
type Layout struct {
	Width, Height int

	BoxX0, BoxY0 int
	BoxX1, BoxY1 int
	BoxWidth     int
	BoxHeight    int
	BoxRadius    int
	BoxOpacity   uint8
	Padding      int

	TitleX, TitleY       int
	SubtitleX, SubtitleY int

	SeparatorY         int
	SeparatorThickness int

	TitleFontSize    float64
	SubtitleFontSize float64
}

const (
	// Target dimensions for the output wallpaper.
	TargetWidth  = 3840
	TargetHeight = 2160

	boxWidthPercent   = 48
	paddingPercent    = 5
	radiusDivisor     = 9 // relative to smaller box dimension
	lineThicknessDiv  = 160
	boxOpacityDefault = 200
)

// ComputeLayoutForText derives layout from image size and measured text using font metrics.
func ComputeLayoutForText(width, height int, titleFace, subtitleFace font.Face, title, subtitle string) (Layout, error) {
	if width <= 0 || height <= 0 {
		width = TargetWidth
		height = TargetHeight
	}
	if titleFace == nil || subtitleFace == nil {
		return Layout{}, fmt.Errorf("layout: font face is nil")
	}

	titleAdvance := font.MeasureString(titleFace, title).Ceil()
	subAdvance := font.MeasureString(subtitleFace, subtitle).Ceil()
	titleMetrics := titleFace.Metrics()
	subMetrics := subtitleFace.Metrics()

	titleHeight := (titleMetrics.Ascent + titleMetrics.Descent).Ceil()
	subtitleHeight := (subMetrics.Ascent + subMetrics.Descent).Ceil()

	padding := maxInt(14, minInt(width, height)*paddingPercent/100)
	contentWidth := maxInt(titleAdvance, subAdvance)
	defaultBoxWidth := width * boxWidthPercent / 100
	boxWidth := maxInt(defaultBoxWidth, contentWidth+padding*2)

	lineThickness := maxInt(2, height/lineThicknessDiv)
	gapAfterTitle := maxInt(padding/3, lineThickness)
	gapAfterSeparator := padding / 2

	boxHeight := padding + titleHeight + gapAfterTitle + lineThickness + gapAfterSeparator + subtitleHeight + padding
	boxX0 := (width - boxWidth) / 2
	boxY0 := (height - boxHeight) / 2
	boxX1 := boxX0 + boxWidth
	boxY1 := boxY0 + boxHeight

	radius := maxInt(10, minInt(boxWidth, boxHeight)/radiusDivisor)

	titleX := boxX0 + (boxWidth-titleAdvance)/2
	titleY := boxY0 + padding + titleMetrics.Ascent.Ceil()
	separatorY := boxY0 + padding + titleHeight + gapAfterTitle + lineThickness/2
	subtitleX := boxX0 + (boxWidth-subAdvance)/2
	subtitleY := separatorY + lineThickness/2 + gapAfterSeparator + subMetrics.Ascent.Ceil()

	return Layout{
		Width:  width,
		Height: height,

		BoxX0:              boxX0,
		BoxY0:              boxY0,
		BoxX1:              boxX1,
		BoxY1:              boxY1,
		BoxWidth:           boxWidth,
		BoxHeight:          boxHeight,
		BoxRadius:          radius,
		BoxOpacity:         boxOpacityDefault,
		Padding:            padding,
		SeparatorY:         separatorY,
		SeparatorThickness: lineThickness,

		TitleX: titleX,
		TitleY: titleY,

		SubtitleX: subtitleX,
		SubtitleY: subtitleY,

		TitleFontSize:    float64(titleMetrics.Ascent.Ceil() + titleMetrics.Descent.Ceil()),
		SubtitleFontSize: float64(subMetrics.Ascent.Ceil() + subMetrics.Descent.Ceil()),
	}, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
