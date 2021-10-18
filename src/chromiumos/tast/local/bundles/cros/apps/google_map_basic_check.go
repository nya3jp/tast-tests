// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/apps/webstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleMapBasicCheck,
		Desc:         "Webstore app google map basic functionality check",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromeLoggedIn",
	})
}

func GoogleMapBasicCheck(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var (
		urlAppName = "google-maps-platform-api"
		urlAppID   = "mlikepnkghhlnkgeejmlkfeheihlehne"
		appURL     = fmt.Sprintf("https://chrome.google.com/webstore/detail/%s/%s", urlAppName, urlAppID)
		startPoint = "Bengaluru"
		endPoint   = "Goa"
		appName    = "Google Maps Platform API Checker"
	)

	const (
		iter           = 5
		imageThreshold = 10
	)

	// App Install parameters
	app := webstore.App{Name: appName,
		URL:           appURL,
		VerifyText:    "Remove from Chrome",
		AddRemoveText: "Add to Chrome",
		ConfirmText:   "Add extension",
	}

	s.Logf("Installing %q app", appName)
	if err := webstore.UpgradeWebstoreApp(ctx, cr, tconn, app); err != nil {
		s.Fatal("Failed to install webapp: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		// App Uninstall parameters
		app = webstore.App{Name: appName,
			URL:           appURL,
			VerifyText:    "Add to Chrome",
			AddRemoveText: "Remove from Chrome",
			ConfirmText:   "Remove",
		}

		s.Logf("Uninstalling %q app", appName)
		if err := webstore.UpgradeWebstoreApp(ctx, cr, tconn, app); err != nil {
			s.Fatal("Failed to uninstall webapp: ", err)
		}
	}(cleanupCtx)

	googleMapURL := "https://www.google.com/maps"
	conn, err := cr.NewConn(ctx, googleMapURL)
	if err != nil {
		s.Fatal("Failed to open google map: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	//Google map zoom-in and zoom-out
	s.Log("Zooming In")
	for i := 0; i <= iter; i++ {
		zoomIn := "document.getElementById('widget-zoom-in').click()"
		if err := mapFunctionalities(ctx, conn, zoomIn); err != nil {
			s.Fatal("Failed to zoom in: ", err)
		}
	}

	s.Log("Zooming out")
	for i := 0; i <= iter; i++ {
		zoomOut := "document.getElementById('widget-zoom-out').click()"
		if err := mapFunctionalities(ctx, conn, zoomOut); err != nil {
			s.Fatal("Failed to zoom out: ", err)
		}
	}

	earthScreenshot := filepath.Join(s.OutDir(), "earth.png")
	mapScreenshot := filepath.Join(s.OutDir(), "map.png")

	screenshot.Capture(ctx, mapScreenshot)

	//Switching between earth view and map view.
	s.Log("Switching to Earth view")
	if err := switchMapView(ctx, conn); err != nil {
		s.Fatal("Failed to switch to Earth view: ", err)
	}

	if err := waitForMapElement(ctx, conn); err != nil {
		s.Fatal("Failed to wait for map element: ", err)
	}

	screenshot.Capture(ctx, earthScreenshot)
	imageDiff, err := getImagesDifference(ctx, mapScreenshot, earthScreenshot)
	if err != nil {
		s.Fatal("Failed to get image differences: ", err)
	}

	if !(int(imageDiff) > imageThreshold) {
		s.Fatal("Failed screenshot comparison before and after switching map is similar")
	}

	s.Log("Switching to Map view")
	if err := switchMapView(ctx, conn); err != nil {
		s.Fatal("Failed to switch to Map view: ", err)
	}

	if err := waitForMapElement(ctx, conn); err != nil {
		s.Fatal("Failed to wait for map element: ", err)
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
	if err := mapFunctionalities(ctx, conn, directions); err != nil {
		s.Fatal("Failed to click on searchbox direction button: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard eventwriter: ", err)
	}

	if err := enterString(ctx, kb, startPoint); err != nil {
		s.Fatal("Failed to enter source point in directions searchbox: ", err)
	}

	if err := enterString(ctx, kb, endPoint); err != nil {
		s.Fatal("Failed to enter destination point in directions searchbox: ", err)
	}

	// Closing map directions searchbox
	closeDirection := `document.querySelector('[aria-label="Close directions"]').click()`
	if err := mapFunctionalities(ctx, conn, closeDirection); err != nil {
		s.Fatal("Failed to close searchbox direction: ", err)
	}
}

func loadJpeg(ctx context.Context, filename string) (image.Image, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %q file", filename)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode %q image file", filename)
	}
	return img, nil
}

func difference(ctx context.Context, a, b uint32) int64 {
	if a > b {
		return int64(a - b)
	}
	return int64(b - a)
}

func getImagesDifference(ctx context.Context, imgFile1, imgFile2 string) (float64, error) {
	vtimgFile1, err := loadJpeg(ctx, imgFile1)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to load and decode %q image file", imgFile1)
	}

	vtimgFile2, err := loadJpeg(ctx, imgFile2)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to load and decode %q image file", imgFile2)
	}

	b := vtimgFile1.Bounds()
	var sum int64

	// Here RGB stands for Red, Green and Blue color respectively.
	// Takes x and y bound values from minimum to maximum of given images.
	// Gets difference of RGB color for given images at each x and y axis value,
	// adds differences to the 'sum' variable of each R, G and B color,
	// then gives the total difference between provided images.
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r1, g1, b1, _ := vtimgFile1.At(x, y).RGBA()
			r2, g2, b2, _ := vtimgFile2.At(x, y).RGBA()
			sum += difference(ctx, r1, r2)
			sum += difference(ctx, g1, g2)
			sum += difference(ctx, b1, b2)
		}
	}
	nPixels := (b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y)
	imgDiffer := float64(sum*100) / (float64(nPixels) * 0xffff * 3)
	return imgDiffer, nil
}

func switchMapView(ctx context.Context, conn *chrome.Conn) error {
	earthMapSwitch := "document.querySelectorAll('[aria-labelledby=widget-minimap-icon-overlay]')[0].click()"
	if err := mapFunctionalities(ctx, conn, earthMapSwitch); err != nil {
		return errors.Wrap(err, "failed to get earth map switching element")
	}
	return nil
}

func waitForMapElement(ctx context.Context, conn *chrome.Conn) error {
	showImagery := `document.querySelector('[aria-label="Show imagery"]').textContent`
	if err := mapFunctionalities(ctx, conn, showImagery); err != nil {
		return errors.Wrap(err, "failed to get show imagery element")
	}
	return nil
}

func mapFunctionalities(ctx context.Context, conn *chrome.Conn, getElement string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, getElement, nil); err != nil {
			return errors.Wrap(err, "failed to get element")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 10 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to find elemet in given timeout")
	}
	return nil
}

func enterString(ctx context.Context, kb *input.KeyboardEventWriter, textToType string) error {
	testing.ContextLogf(ctx, "Entering %q in Maps direction searchbox", textToType)
	if err := kb.Type(ctx, textToType); err != nil {
		return errors.Wrapf(err, "failed to type %q in direction searchbox", textToType)
	}
	if err := kb.Accel(ctx, "enter"); err != nil {
		return errors.Wrap(err, "failed to press enter key")
	}
	return nil
}
