// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/graphics/webstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleMapBasicCheck,
		Desc:         "Verifies webstore google map app basic functionality check",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
	})
}

func GoogleMapBasicCheck(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)

	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}

	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)

	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	var (
		urlAppName = "google-maps-platform-api"
		urlAppID   = "mlikepnkghhlnkgeejmlkfeheihlehne"
		appURL     = fmt.Sprintf("https://chrome.google.com/webstore/detail/%s/%s", urlAppName, urlAppID)
		startPoint = "Bengaluru"
		endPoint   = "Goa"
		appName    = "Google Maps Platform API Checker"
	)

	// App Install parameters
	app := webstore.App{Name: appName,
		URL: appURL, InstalledTxt: "Remove from Chrome",
		AddTxt:     "Add to Chrome",
		ConfirmTxt: "Add extension",
	}

	s.Logf("Installing %s app", appName)

	if err := webstore.InstallWebstoreApp(ctx, cr, tconn, app); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		// App Uninstall parameters
		app = webstore.App{Name: appName,
			URL: appURL, InstalledTxt: "Add to Chrome",
			AddTxt:     "Remove from Chrome",
			ConfirmTxt: "Remove",
		}

		s.Logf("Uninstalling %s app", appName)

		if err := webstore.InstallWebstoreApp(ctx, cr, tconn, app); err != nil {
			s.Fatal("Failed to un-install app: ", err)
		}
	}(ctx)

	googleMapURL := "https://www.google.com/maps"
	conn, err := cr.NewConn(ctx, googleMapURL)

	if err != nil {
		s.Fatal("Failed to open google map: ", err)
	}

	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Performs map view switching and validates for expected view.
	switchMapView := func(ctx context.Context) error {
		earthMapSwitch := "document.querySelectorAll('[aria-labelledby=widget-minimap-icon-overlay]')[0].click()"

		if err := testing.Poll(ctx, func(ctx context.Context) error {

			if err := conn.Eval(ctx, earthMapSwitch, nil); err != nil {
				return errors.Wrap(err, "failed to get map switch element")
			}

			return nil
		}, &testing.PollOptions{
			Timeout: 10 * time.Second,
		}); err != nil {
			return errors.Wrap(err, "failed to switch between earth-view/map-view")
		}

		return nil
	}

	//Google map zoom-in and zoom-out
	s.Log("Zooming In")
	const iter = 5

	for i := 0; i <= iter; i++ {
		zoomIn := "document.getElementById('widget-zoom-in').click()"

		if err := testing.Poll(ctx, func(ctx context.Context) error {

			if err := conn.Eval(ctx, zoomIn, nil); err != nil {
				return errors.Wrap(err, "failed to get zoom-in element")
			}
			return nil
		}, &testing.PollOptions{
			Timeout: 10 * time.Second,
		}); err != nil {
			s.Fatal("Failed to zoom in: ", err)
		}
	}

	s.Log("Zooming out")
	for i := 0; i <= iter; i++ {
		zoomOut := "document.getElementById('widget-zoom-out').click()"

		if err := testing.Poll(ctx, func(ctx context.Context) error {

			if err := conn.Eval(ctx, zoomOut, nil); err != nil {
				return errors.Wrap(err, "failed to get zoom-out element")
			}

			return nil
		}, &testing.PollOptions{
			Timeout: 10 * time.Second,
		}); err != nil {
			s.Fatal("Failed to zoom out: ", err)
		}
	}

	//Switching between earth view and map view.
	s.Log("Switching to Earth view")

	earthScreenshot := filepath.Join(s.OutDir(), "earth.png")
	mapScreenshot := filepath.Join(s.OutDir(), "map.png")
	// Image difference threshold value
	const imageThreshold = 10
	screenshot.Capture(ctx, mapScreenshot)

	if err := switchMapView(ctx); err != nil {
		s.Fatal("Failed to switch to Earth view: ", err)
	}

	// Expected sleep, for map to load.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	screenshot.Capture(ctx, earthScreenshot)
	imageDiff, err := getImagesDifference(ctx, mapScreenshot, earthScreenshot)

	if err != nil {
		s.Fatal("Failed to get image differences: ", err)
	}

	if !(int(imageDiff) > imageThreshold) {
		s.Fatalf("Failed screenshot comparison expecting difference greater than %d%%; but got: %d%%", imageThreshold, (int(imageDiff)))
	}

	s.Log("Switching to Map view")

	if err := switchMapView(ctx); err != nil {
		s.Fatal("Failed to switch to Map view: ", err)
	}

	// Expected sleep, for map to load.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	screenshot.Capture(ctx, mapScreenshot)
	imageDiff, err = getImagesDifference(ctx, mapScreenshot, earthScreenshot)

	if err != nil {
		s.Fatal("Failed to get image differences: ", err)
	}

	if !(int(imageDiff) > imageThreshold) {
		s.Fatalf("Failed screenshot comparison expecting difference greater than %d%%; but got: %d%%", imageThreshold, (int(imageDiff)))
	}

	// Performing find directions using Google Map.
	directions := `document.querySelector('[aria-label="Directions"]').click()`

	if err := testing.Poll(ctx, func(ctx context.Context) error {

		if err := conn.Eval(ctx, directions, nil); err != nil {
			return errors.Wrap(err, "failed to get directions element")
		}

		return nil
	}, &testing.PollOptions{
		Timeout: 10 * time.Second,
	}); err != nil {
		s.Fatal("Failed to click on searchbox direction button: ", err)
	}

	kb, _ := input.Keyboard(ctx)
	s.Logf("Entering %q as start-point in Maps direction", startPoint)

	if err := kb.Type(ctx, startPoint); err != nil {
		s.Fatalf("Failed to type %s in direction searchbox: %v", startPoint, err)
	}

	if err := kb.Accel(ctx, "enter"); err != nil {
		s.Fatal("Failed to press 'Enter' key: ", err)
	}

	s.Logf("Entering %q as end-point in Maps direction", endPoint)

	if err := kb.Type(ctx, endPoint); err != nil {
		s.Fatalf("Failed to type %s in direction searchbox: %v", endPoint, err)
	}

	if err := kb.Accel(ctx, "enter"); err != nil {
		s.Fatal("Failed to press 'Enter' key: ", err)
	}

	closeDirection := `document.querySelector('[aria-label="Close directions"]').click()`
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		if err := conn.Eval(ctx, closeDirection, nil); err != nil {
			return errors.Wrap(err, "failed to get closeDirection element")
		}

		return nil
	}, &testing.PollOptions{
		Timeout: 10 * time.Second,
	}); err != nil {
		s.Fatal("Failed to close searchbox direction: ", err)
	}
}

func getImagesDifference(ctx context.Context, imgFile1, imgFile2 string) (float64, error) {
	loadJpeg := func(filename string) (image.Image, error) {
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

	vtimgFile1, err := loadJpeg(imgFile1)

	if err != nil {
		return 0.0, errors.Wrap(err, "failed to load and decode image")
	}

	vtimgFile2, err := loadJpeg(imgFile2)

	if err != nil {
		return 0.0, errors.Wrap(err, "failed to load and decode image")
	}

	b := vtimgFile1.Bounds()
	var sum int64

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r1, g1, b1, _ := vtimgFile1.At(x, y).RGBA()
			r2, g2, b2, _ := vtimgFile2.At(x, y).RGBA()
			sum += difference(r1, r2)
			sum += difference(g1, g2)
			sum += difference(b1, b2)
		}
	}
	nPixels := (b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y)
	testing.ContextLogf(ctx, "Image difference: %f%%", float64(sum*100)/(float64(nPixels)*0xffff*3))
	return float64(sum*100) / (float64(nPixels) * 0xffff * 3), nil
}
