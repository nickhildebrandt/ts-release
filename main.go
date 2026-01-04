package main

import (
	"fmt"
	"os"
	"time"

	"github.com/nickhildebrandt/ts-release/internal/install"
	"github.com/nickhildebrandt/ts-release/internal/wallpaper"
)

func main() {
	if len(os.Args) != 3 {
		usage()
		os.Exit(1)
	}

	targetName := os.Args[1]
	rootFS := os.Args[2]

	if targetName == "" {
		usage()
		os.Exit(1)
	}

	info, err := os.Stat(rootFS)
	if err != nil || !info.IsDir() {
		usage()
		os.Exit(1)
	}

	buildID := time.Now().UTC().Format(time.RFC3339)

	img, err := wallpaper.Generate(targetName, buildID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := install.Install(rootFS, img, buildID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: ts-release <target-name> <rootfs-dir>")
}
