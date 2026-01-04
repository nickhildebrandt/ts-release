# ts-release

`ts-release` is the release-artifact generator for TSSH.

Given a *target name* and a *root filesystem directory*, it:

- Computes a build release number (UTC timestamp, RFC3339)
- Fetches a QHD “nature” wallpaper image from Wallhaven
- Renders an overlay (title + build ID) onto that image
- Installs the generated artifacts into the provided rootfs tree:
	- a BMP splash image for Systemd UKI boot splash usage
	- a JPEG background image for GNOME desktop
	- an `etc` release-info file

## Quick start

Build:

```bash
go build -o ts-release .
```

Generate artifacts into an existing rootfs directory:

```bash
mkdir rootfs
./ts-release "my-target" rootfs
```

This will write:

- `./rootfs/boot/splash.bmp`
- `./rootfs/usr/share/backgrounds/tssh/background.jpg`
- `./rootfs/etc/tssh.build`

## CLI

The program expects exactly two arguments:

```text
<target-name> <rootfs-dir>
```

Notes:

- The `rootfs-dir` must already exist and must be a directory. If it does not exist, the program fails.
- If `rootfs-dir` exists but is empty, the program bootstraps the expected subfolders (similar to a fresh post `depth-bootstrap` filesystem) and then writes the artifacts.

## What gets generated (and where)

The installer writes three files into the provided rootfs:

```text
<rootfs-dir>/
├── boot/
│   └── splash.bmp
├── etc/
│   └── tssh.build
└── usr/
		└── share/
				└── backgrounds/
						└── tssh/
								└── background.jpg
```

File details:

- `boot/splash.bmp`
	- Format: BMP
	- Intended use: boot splash artwork (e.g. consumed by your UKI build pipeline)
- `usr/share/backgrounds/tssh/background.jpg`
	- Format: JPEG (quality 92)
	- Intended use: GNOME desktop wallpaper
- `etc/tssh.build`
	- Content: build release number as a single line, `UTC RFC3339` (e.g. `2026-01-04T13:35:13Z`)

The repository also includes a `test/` folder that mirrors the expected installed locations so you can run the tool against it locally.

## Build release number

The build release number is:

- Generated at runtime as `time.Now().UTC().Format(time.RFC3339)`
- Used as the subtitle text rendered into the image
- Written verbatim to `etc/tssh.build` (with a trailing newline)

## Image source (“nature”)

Background images are fetched from Wallhaven using its public API:

- Endpoint: `https://wallhaven.cc/api/v1/search`
- Query keyword: `nature` (this is the key theme)
- Categories: `100` (General)
- Purity: `100` (SFW)
- Sorting: `random`
- Resolution: fixed to QHD (3840×2160)

The tool downloads the first search result’s direct image URL, then decodes it (JPEG/PNG/GIF supported via Go’s image decoders).

Because this depends on an external service:

- You need internet access when running the generator.
- The run can fail if Wallhaven returns no suitable image for the requested resolution or returns a non-2xx HTTP status.

## Render/layout design (QHD)

The output wallpaper size is fixed to QHD:

- Width: 3840
- Height: 2160

### Background scaling

The fetched image is scaled and cropped to fill the canvas:

- Scale factor: `max(targetW/srcW, targetH/srcH)` (cover)
- Resampling: Catmull-Rom
- Crop: centered (equal trim on opposite sides)

### Text content

The renderer composes two lines:

- Title: `TSSH <target-name>` (or just `TSSH` if the target name is blank)
- Subtitle: the build ID (or `build unknown` if missing)

### Typography

Fonts are embedded into the Go binary via `go:embed` and loaded with `golang.org/x/image/font/opentype`:

- Bold: DejaVu Sans Bold (title)
- Regular: DejaVu Sans (subtitle)
- Title font size: `0.06 * TargetHeight`
- Subtitle font size: `0.036 * TargetHeight`

### Overlay box geometry

The overlay is a centered rounded rectangle with a separator line:

- Box width starts at `48%` of image width and grows if needed to fit the longest text width plus padding
- Padding: `max(14px, 5% of min(width, height))`
- Corner radius: `max(10px, min(boxW, boxH)/9)`
- Box opacity: 200 (out of 255)
- Separator thickness: `max(2px, height/160)`

Everything (box, title, separator, subtitle) is centered both horizontally and vertically.

### Max target name length (practical limit: ~26)

The renderer enforces a maximum *pixel width* for each line of text based on the image width.
For QHD, this effectively limits the title to what fits within an inner safe area (image width minus ~15% margins on each side).

In practice, with the current font sizing and DejaVu Sans Bold, a safe guideline is:

- Keep `<target-name>` to **≤ 26 characters** for QHD.

This is not a hard “character counter” rule: wide characters (e.g. `W`) reduce the maximum, and narrow characters allow more.
If text is too long, the program fails with an error asking you to reduce the text.

## Dependencies / libraries

- Go standard library (`image`, `image/jpeg`, `net/http`, `time`, etc.)
- `golang.org/x/image`
	- `bmp` encoding for `boot/splash.bmp`
	- high-quality scaling (`draw.CatmullRom`)
	- font rendering and TTF parsing (`font`, `opentype`)

## Development

### Dev setup (new machine)

Required:

- Go toolchain matching `go.mod` (currently Go 1.24.x)

Recommended / typically required on Linux:

- CA certificates (`ca-certificates` package on most distros), because the generator fetches images over HTTPS
- A normal libc runtime (this is a regular dynamically linked Linux ELF on typical builds)

### Build

```bash
go build -o ./bin/ts-release .
```

### Run all tests

```bash
go test ./...
```

### Useful local test commands

Generate into the included `test/` tree:

```bash
go build -o ./bin/ts-release .
./bin/ts-release "test" ./test
```

Verify expected outputs exist:

```bash
test -f ./test/boot/splash.bmp
test -f ./test/usr/share/backgrounds/tssh/background.jpg
test -f ./test/etc/tssh.build
cat ./test/etc/tssh.build
```

## Fonts

The binary embeds DejaVu Sans TTF files from:

- `internal/wallpaper/fonts/DejaVuSans.ttf`
- `internal/wallpaper/fonts/DejaVuSans-Bold.ttf`

If you need to replace the fonts, keep valid TTF files at the embedded paths above; otherwise the build will fail.
