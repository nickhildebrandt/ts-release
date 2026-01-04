package install

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
)

// sampleImage builds a small deterministic test image with multiple colors.
// It is used as input for Install tests and must be encodable/decodable.
func sampleImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	img.Set(1, 0, color.RGBA{0, 255, 0, 255})
	img.Set(2, 0, color.RGBA{0, 0, 255, 255})
	img.Set(0, 1, color.RGBA{10, 20, 30, 255})
	img.Set(1, 1, color.RGBA{40, 50, 60, 255})
	img.Set(2, 1, color.RGBA{70, 80, 90, 255})
	return img
}

// TestInstall_SucceedsAndWritesExpectedPaths verifies that Install creates all expected files and that formats are decodable.
// The test fails if paths are missing or the written BMP/JPEG is not valid.
func TestInstall_SucceedsAndWritesExpectedPaths(t *testing.T) {
	root := t.TempDir()
	img := sampleImage()
	buildID := "build-123"

	if err := Install(root, img, buildID); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	bmpPath := filepath.Join(root, "boot", "splash.bmp")
	jpgPath := filepath.Join(root, "usr", "share", "backgrounds", "tssh", "background.jpg")
	buildPath := filepath.Join(root, "etc", "tssh.build")

	if _, err := os.Stat(bmpPath); err != nil {
		t.Fatalf("expected %s to exist: %v", bmpPath, err)
	}
	if _, err := os.Stat(jpgPath); err != nil {
		t.Fatalf("expected %s to exist: %v", jpgPath, err)
	}
	if _, err := os.Stat(buildPath); err != nil {
		t.Fatalf("expected %s to exist: %v", buildPath, err)
	}

	{
		f, err := os.Open(bmpPath)
		if err != nil {
			t.Fatalf("open bmp: %v", err)
		}
		defer f.Close()
		if _, err := bmp.Decode(f); err != nil {
			t.Fatalf("decode bmp: %v", err)
		}
	}
	{
		f, err := os.Open(jpgPath)
		if err != nil {
			t.Fatalf("open jpg: %v", err)
		}
		defer f.Close()
		if _, err := jpeg.Decode(f); err != nil {
			t.Fatalf("decode jpg: %v", err)
		}
	}

	data, err := os.ReadFile(buildPath)
	if err != nil {
		t.Fatalf("read build file: %v", err)
	}
	if string(data) != buildID+"\n" {
		t.Fatalf("unexpected build file content %q", string(data))
	}
}

// TestInstall_MissingRootFS_Error expects an error when the rootfs directory does not exist.
// It also checks that the error message reflects the missing-path case.
func TestInstall_MissingRootFS_Error(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if err := Install(missing, sampleImage(), "b"); err == nil {
		t.Fatalf("expected error")
	} else if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

// TestInstall_RootFSIsFile_Error expects an error when the rootfs path points to a file instead of a directory.
// This ensures Install does not silently write into an invalid target.
func TestInstall_RootFSIsFile_Error(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "rootfs")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := Install(p, sampleImage(), "b"); err == nil {
		t.Fatalf("expected error")
	} else if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

// TestInstall_ImageNil_Error expects an error when Install is called with a nil image.
// This prevents later panics in the encoder paths.
func TestInstall_ImageNil_Error(t *testing.T) {
	root := t.TempDir()
	if err := Install(root, nil, "b"); err == nil {
		t.Fatalf("expected error")
	} else if !strings.Contains(err.Error(), "image is nil") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

// TestInstall_EmptyBuildID_CurrentBehavior documents that an empty build ID is currently allowed.
// It expects that exactly a newline is written to the metadata file.
func TestInstall_EmptyBuildID_CurrentBehavior(t *testing.T) {
	root := t.TempDir()
	if err := Install(root, sampleImage(), ""); err != nil {
		t.Fatalf("expected success with empty buildID, got error: %v", err)
	}
	buildPath := filepath.Join(root, "etc", "tssh.build")
	data, err := os.ReadFile(buildPath)
	if err != nil {
		t.Fatalf("read build file: %v", err)
	}
	if string(data) != "\n" {
		t.Fatalf("unexpected build file content %q", string(data))
	}
}

// TestInstall_OverwritesExistingFiles ensures that Install overwrites existing output files.
// The test fails if old contents remain or the new files are not decodable.
func TestInstall_OverwritesExistingFiles(t *testing.T) {
	root := t.TempDir()
	img := sampleImage()

	bmpPath := filepath.Join(root, "boot", "splash.bmp")
	jpgPath := filepath.Join(root, "usr", "share", "backgrounds", "tssh", "background.jpg")
	buildPath := filepath.Join(root, "etc", "tssh.build")

	if err := os.MkdirAll(filepath.Dir(bmpPath), 0o755); err != nil {
		t.Fatalf("mkdir bmp dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(jpgPath), 0o755); err != nil {
		t.Fatalf("mkdir jpg dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(buildPath), 0o755); err != nil {
		t.Fatalf("mkdir build dir: %v", err)
	}
	if err := os.WriteFile(bmpPath, []byte("garbage"), 0o644); err != nil {
		t.Fatalf("write bmp garbage: %v", err)
	}
	if err := os.WriteFile(jpgPath, []byte("garbage"), 0o644); err != nil {
		t.Fatalf("write jpg garbage: %v", err)
	}
	if err := os.WriteFile(buildPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write build old: %v", err)
	}

	if err := Install(root, img, "new-build"); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	data, err := os.ReadFile(buildPath)
	if err != nil {
		t.Fatalf("read build file: %v", err)
	}
	if string(data) != "new-build\n" {
		t.Fatalf("unexpected build file content %q", string(data))
	}

	{
		f, err := os.Open(bmpPath)
		if err != nil {
			t.Fatalf("open bmp: %v", err)
		}
		defer f.Close()
		if _, err := bmp.Decode(f); err != nil {
			t.Fatalf("decode bmp: %v", err)
		}
	}
	{
		f, err := os.Open(jpgPath)
		if err != nil {
			t.Fatalf("open jpg: %v", err)
		}
		defer f.Close()
		if _, err := jpeg.Decode(f); err != nil {
			t.Fatalf("decode jpg: %v", err)
		}
	}
}

// TestInstall_ReadOnlyRootFS_Error expects an error when the rootfs is not writable.
// This verifies that Install propagates write failures.
func TestInstall_ReadOnlyRootFS_Error(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	err := Install(root, sampleImage(), "b")
	if err == nil {
		t.Fatalf("expected error on read-only rootfs")
	}
}
