// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VT2ShellStress,
		Desc:         "Switch between VT-2 shell and GUI multiple times",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"iterations"}, // Number of iterations to test.
		Attr:         []string{"group:mainline", "informational"},
	})
}

const (
	waitTime                   = 2 * time.Second
	differencePercentThreshold = 5
)

func intVar(s *testing.State, name string, defaultValue int) int {
	str, ok := s.Var(name)
	if !ok {
		return defaultValue
	}

	val, err := strconv.Atoi(str)
	if err != nil {
		s.Fatalf("Failed to parse integer variable %v: %v", name, err)
	}
	return val
}

// VT2ShellStress will switch between VT-2 shell and GUI for multiple times.
func VT2ShellStress(ctx context.Context, s *testing.State) {
	const defaultIterations = 1
	iterations := intVar(s, "iterations", defaultIterations)
	s.Logf("No. of iterations: %d", iterations)

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in by Chrome: ", err)
	}
	defer cr.Close(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatalf("Failed to open the keyboard:%v", err)
	}

	openVT := func(vt string) {
		keyboardKey := "ctrl+alt+back"
		if vt == "2" {
			keyboardKey = "ctrl+alt+refresh"
		}
		s.Logf("Opening VT-%v by pressing %s", vt, keyboardKey)
		if err := kb.Accel(ctx, keyboardKey); err != nil {
			s.Fatalf("Failed to press key: %s, error: %v", keyboardKey, err)
		}
		testing.Sleep(ctx, waitTime) // Allowing some wait time for switching to happen.
	}

	defer func(ctx context.Context) {
		openVT("1")
	}(ctx)

	loadImage := func(filename string) (image.Image, error) {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			return nil, err
		}
		return img, nil
	}

	difference := func(a, b uint32) int64 {
		if a > b {
			return int64(a - b)
		}
		return int64(b - a)
	}

	getPercentDifference := func(file1, file2 string) float64 {
		vtFile1, err := loadImage(file1)
		if err != nil {
			s.Fatal("Failed to load the image: ", err)
		}
		vtFile2, err := loadImage(file2)
		if err != nil {
			s.Fatal("Failed to load the image: ", err)
		}
		b := vtFile1.Bounds()
		var sum int64
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r1, g1, b1, _ := vtFile1.At(x, y).RGBA()
				r2, g2, b2, _ := vtFile2.At(x, y).RGBA()
				sum += difference(r1, r2)
				sum += difference(g1, g2)
				sum += difference(b1, b2)
			}
		}
		nPixels := (b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y)
		return float64(sum*100) / (float64(nPixels) * 0xffff * 3)
	}

	currentVT1Screenshot := filepath.Join(s.OutDir(), "VT-1.png")
	screenshot.Capture(ctx, currentVT1Screenshot)
	for i := 0; i < iterations; i++ {
		// Switch to VT-2 shell and capture screenshot. Compare this screenshot with VT-1 screenshot.
		openVT("2")
		currentVT2Screenshot := filepath.Join(s.OutDir(), "VT-2_"+strconv.Itoa(i)+".png")
		screenshot.Capture(ctx, currentVT2Screenshot)
		diff := int(getPercentDifference(currentVT1Screenshot, currentVT2Screenshot))
		if diff < differencePercentThreshold {
			s.Fatalf("Failed to switch to VT2 shell in iteration %d; VT1 and VT2 screenshots differ by %d percent", (i + 1), diff)
		}
		// Switch to VT-1 shell and capture screenshot. Compare this screenshot with VT-2 screenshot.
		openVT("1")
		currentVT1Screenshot := filepath.Join(s.OutDir(), "VT-1_"+strconv.Itoa(i)+".png")
		screenshot.Capture(ctx, currentVT1Screenshot)
		diff = int(getPercentDifference(currentVT1Screenshot, currentVT2Screenshot))
		if diff < differencePercentThreshold {
			s.Fatalf("Failed to switch to VT1 shell in iteration %d; VT1 and VT2 screenshots differ by %d percent", (i + 1), diff)
		}
	}
}
