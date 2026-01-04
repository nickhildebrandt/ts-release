package wallpaper

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"math"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed fonts/DejaVuSans.ttf
var regularFontData []byte

//go:embed fonts/DejaVuSans-Bold.ttf
var boldFontData []byte

// Render composes the final wallpaper using the given background and text labels.
func Render(bg image.Image, targetName string, buildID string) (*image.RGBA, error) {
	if bg == nil {
		return nil, fmt.Errorf("render: background is nil")
	}

	// Build text first to measure with the actual faces.
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

	titleSize := float64(TargetHeight) * 0.06
	subtitleSize := float64(TargetHeight) * 0.036

	titleFace, err := loadFace(boldFontData, titleSize)
	if err != nil {
		return nil, fmt.Errorf("render: load title font: %w", err)
	}

	subtitleFace, err := loadFace(regularFontData, subtitleSize)
	if err != nil {
		return nil, fmt.Errorf("render: load subtitle font: %w", err)
	}

	layout, err := ComputeLayoutForText(TargetWidth, TargetHeight, titleFace, subtitleFace, title, subtitle)
	if err != nil {
		return nil, err
	}

	backgroundLayer, err := resizeAndCrop(bg, layout.Width, layout.Height)
	if err != nil {
		return nil, err
	}

	canvas := image.NewRGBA(image.Rect(0, 0, layout.Width, layout.Height))
	stddraw.Draw(canvas, canvas.Bounds(), backgroundLayer, image.Point{}, stddraw.Src)

	boxColor := color.NRGBA{R: 12, G: 16, B: 24, A: layout.BoxOpacity}
	overlay := image.NewRGBA(canvas.Bounds())
	drawRoundedRect(overlay, image.Rect(layout.BoxX0, layout.BoxY0, layout.BoxX1, layout.BoxY1), layout.BoxRadius, boxColor)
	stddraw.Draw(canvas, overlay.Bounds(), overlay, image.Point{}, stddraw.Over)

	lineColor := color.NRGBA{R: 255, G: 255, B: 255, A: 140}
	titleWidth := font.MeasureString(titleFace, title).Ceil()
	subtitleWidth := font.MeasureString(subtitleFace, subtitle).Ceil()
	longestTextWidth := maxInt(titleWidth, subtitleWidth)
	drawSeparator(canvas, layout, lineColor, longestTextWidth)

	textColor := color.NRGBA{R: 241, G: 243, B: 246, A: 255}
	secondaryText := color.NRGBA{R: 210, G: 214, B: 222, A: 255}

	maxTextWidth, err := maxTextWidthForImage(layout.Width)
	if err != nil {
		return nil, err
	}

	if err := validateTextWidth("title", titleFace, title, maxTextWidth); err != nil {
		return nil, err
	}
	if err := drawText(canvas, titleFace, title, layout.TitleX, layout.TitleY, textColor); err != nil {
		return nil, err
	}
	if err := validateTextWidth("subtitle", subtitleFace, subtitle, maxTextWidth); err != nil {
		return nil, err
	}
	if err := drawText(canvas, subtitleFace, subtitle, layout.SubtitleX, layout.SubtitleY, secondaryText); err != nil {
		return nil, err
	}

	return canvas, nil
}

// Generate is the public entry point that wires fetching, layout, and rendering together.
func Generate(targetName string, buildID string) (*image.RGBA, error) {
	bg, err := FetchBackground(TargetWidth, TargetHeight)
	if err != nil {
		return nil, err
	}
	return Render(bg, targetName, buildID)
}

func resizeAndCrop(src image.Image, width, height int) (*image.RGBA, error) {
	bounds := src.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return nil, fmt.Errorf("render: background has zero area")
	}

	scale := math.Max(float64(width)/float64(bounds.Dx()), float64(height)/float64(bounds.Dy()))
	scaledW := int(math.Ceil(float64(bounds.Dx()) * scale))
	scaledH := int(math.Ceil(float64(bounds.Dy()) * scale))

	scaled := image.NewRGBA(image.Rect(0, 0, scaledW, scaledH))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, bounds, draw.Src, nil)

	offsetX := (scaledW - width) / 2
	offsetY := (scaledH - height) / 2

	cropped := image.NewRGBA(image.Rect(0, 0, width, height))
	stddraw.Draw(cropped, cropped.Bounds(), scaled, image.Point{X: offsetX, Y: offsetY}, stddraw.Src)
	return cropped, nil
}

func loadFace(fontData []byte, size float64) (font.Face, error) {
	parsed, err := opentype.Parse(fontData)
	if err != nil {
		return nil, fmt.Errorf("render: parse font: %w", err)
	}

	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72})
	if err != nil {
		return nil, fmt.Errorf("render: construct font face: %w", err)
	}
	return face, nil
}

func drawRoundedRect(dst *image.RGBA, rect image.Rectangle, radius int, col color.NRGBA) {
	if radius <= 0 {
		stddraw.Draw(dst, rect, image.NewUniform(col), image.Point{}, stddraw.Over)
		return
	}

	radius = minInt(radius, minInt(rect.Dx()/2, rect.Dy()/2))
	// Build a zero-based mask the size of the box to avoid affecting pixels outside the box bounds.
	maskRect := image.Rect(0, 0, rect.Dx(), rect.Dy())
	mask := image.NewAlpha(maskRect)
	fillRoundedMask(mask, radius)
	stddraw.DrawMask(dst, rect, image.NewUniform(col), image.Point{}, mask, image.Point{}, stddraw.Over)
}

func drawSeparator(dst *image.RGBA, layout Layout, col color.NRGBA, textWidth int) {
	lineHeight := layout.SeparatorThickness
	maxWidth := layout.BoxWidth - 2*layout.Padding
	if textWidth > maxWidth {
		textWidth = maxWidth
	}
	extra := maxInt(layout.Padding/4, 10)
	desiredWidth := textWidth + extra
	if desiredWidth > maxWidth {
		desiredWidth = maxWidth
	}
	startX := layout.BoxX0 + (layout.BoxWidth-desiredWidth)/2
	endX := startX + desiredWidth
	lineRect := image.Rect(startX, layout.SeparatorY-lineHeight/2, endX, layout.SeparatorY+lineHeight/2)
	stddraw.Draw(dst, lineRect, image.NewUniform(col), image.Point{}, stddraw.Over)
}

func drawText(dst *image.RGBA, face font.Face, text string, x, y int, col color.NRGBA) error {
	if face == nil {
		return fmt.Errorf("render: font face is nil")
	}
	drawer := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	drawer.DrawString(text)
	return nil
}

func validateTextWidth(label string, face font.Face, text string, maxWidth int) error {
	if maxWidth <= 0 {
		return fmt.Errorf("render: %s text is too long for the selected image resolution, please reduce the text", label)
	}
	advance := font.MeasureString(face, text)
	if advance.Ceil() > maxWidth {
		return fmt.Errorf("render: %s text is too long for the selected image resolution, please reduce the text", label)
	}
	return nil
}

func maxTextWidthForImage(imageWidth int) (int, error) {
	if imageWidth <= 0 {
		return 0, fmt.Errorf("render: invalid image width %d", imageWidth)
	}
	margin := maxInt(24, int(math.Round(float64(imageWidth)*0.15)))
	maxWidth := imageWidth - 2*margin
	if maxWidth <= 0 {
		return 0, fmt.Errorf("render: text is too long for the selected image resolution, please reduce the text")
	}
	return maxWidth, nil
}

func fillRoundedMask(mask *image.Alpha, radius int) {
	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()
	r := radius
	r2 := r * r

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			inside := false

			// Center region (no corner rounding needed).
			if (x >= r && x < w-r) || (y >= r && y < h-r) {
				inside = true
			} else {
				// Corner regions: use circle test.
				dx := 0
				dy := 0
				if x < r {
					dx = r - 1 - x
				} else if x >= w-r {
					dx = x - (w - r)
				}
				if y < r {
					dy = r - 1 - y
				} else if y >= h-r {
					dy = y - (h - r)
				}

				if dx*dx+dy*dy < r2 {
					inside = true
				}
			}

			if inside {
				mask.SetAlpha(b.Min.X+x, b.Min.Y+y, color.Alpha{A: 255})
			}
		}
	}
}
