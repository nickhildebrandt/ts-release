package install

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"golang.org/x/image/bmp"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
)

// Install writes the generated artifacts into the given rootfs and creates missing target directories.
// It returns an error for invalid rootfs paths, a nil image, or any write/encode failure.
func Install(rootFS string, img image.Image, buildID string) error {
	if rootFS == "" {
		return fmt.Errorf("install: rootfs path is empty")
	}

	info, err := os.Stat(rootFS)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("install: rootfs %q does not exist", rootFS)
		}
		return fmt.Errorf("install: stat rootfs: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("install: rootfs %q is not a directory", rootFS)
	}
	if img == nil {
		return fmt.Errorf("install: image is nil")
	}

	bootDir := filepath.Join(rootFS, "boot")
	backgroundDir := filepath.Join(rootFS, "usr", "share", "backgrounds", "tssh")
	etcDir := filepath.Join(rootFS, "etc")

	for _, dir := range []string{bootDir, backgroundDir, etcDir} {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("install: create dir %q: %w", dir, err)
		}
	}

	if err := writeBMP(filepath.Join(bootDir, "splash.bmp"), img); err != nil {
		return err
	}

	if err := writeJPEG(filepath.Join(backgroundDir, "background.jpg"), img); err != nil {
		return err
	}

	if err := writeText(filepath.Join(etcDir, "tssh.build"), buildID+"\n"); err != nil {
		return err
	}

	return nil
}

// writeBMP writes the image as a BMP to the target path and overwrites any existing file.
// It returns an error if the file cannot be opened/created or the BMP encoding fails.
func writeBMP(path string, img image.Image) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerm)
	if err != nil {
		return fmt.Errorf("install: open bmp %q: %w", path, err)
	}
	defer file.Close()

	if err := bmp.Encode(file, img); err != nil {
		return fmt.Errorf("install: encode bmp %q: %w", path, err)
	}
	return nil
}

// writeJPEG writes the image as a JPEG to the target path and overwrites any existing file.
// It returns an error if opening/writing fails or if the JPEG encoding fails.
func writeJPEG(path string, img image.Image) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerm)
	if err != nil {
		return fmt.Errorf("install: open jpeg %q: %w", path, err)
	}
	defer file.Close()

	options := &jpeg.Options{Quality: 92}
	if err := jpeg.Encode(file, img, options); err != nil {
		return fmt.Errorf("install: encode jpeg %q: %w", path, err)
	}
	return nil
}

// writeText writes plain text to a file and overwrites any existing file.
// It returns an error if the file cannot be created or the write fails.
func writeText(path string, content string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerm)
	if err != nil {
		return fmt.Errorf("install: open metadata %q: %w", path, err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("install: write metadata %q: %w", path, err)
	}
	return nil
}
