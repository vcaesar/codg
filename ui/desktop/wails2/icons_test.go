package main

import (
	"bytes"
	"image/png"
	"os"
	"testing"
)

func TestAppIcon(t *testing.T) {
	t.Parallel()
	img, err := png.Decode(bytes.NewReader(appIcon))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 1024 {
		t.Fatalf("icon bounds = %v, want 1024x1024", img.Bounds())
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Fatal("icon corners must be transparent")
	}
	other, err := os.ReadFile("../wails/build/appicon.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(appIcon, other) {
		t.Fatal("Wails v2 and v3 must ship the same Codg logo")
	}
}
