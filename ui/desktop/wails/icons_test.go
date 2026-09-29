package main

import (
	"bytes"
	"image/png"
	"testing"
)

func TestDesktopIcons(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		data []byte
		size int
	}{
		{"application", appIcon, 1024},
		{"tray template", trayTemplateIcon, 44},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			img, err := png.Decode(bytes.NewReader(tt.data))
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != tt.size || img.Bounds().Dy() != tt.size {
				t.Fatalf("icon bounds = %v, want %dx%d", img.Bounds(), tt.size, tt.size)
			}
			if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
				t.Fatal("icon corners must be transparent")
			}
			visible := 0
			for y := 0; y < tt.size; y++ {
				for x := 0; x < tt.size; x++ {
					r, g, b, a := img.At(x, y).RGBA()
					if a > 0 {
						visible++
					}
					if tt.name == "tray template" && (r != 0 || g != 0 || b != 0) {
						t.Fatal("macOS template must be a black alpha mask without a background")
					}
				}
			}
			if visible == 0 {
				t.Fatal("icon must not be empty")
			}
		})
	}
}
