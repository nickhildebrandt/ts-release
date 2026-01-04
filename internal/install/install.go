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

// Install writes pre-generated artifacts into the provided root filesystem.
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
