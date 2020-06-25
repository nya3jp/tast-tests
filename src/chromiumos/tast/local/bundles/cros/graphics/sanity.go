// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Sanity,
		Desc: "Quick sanity check for GL/GLES2",
		Contacts: []string{
			"vsuley@chromium.org",
			"hidehiko@chromium.org", // Tast port author
		},
		Attr: []string{"group:mainline"},
		// TODO(pwang): Remove display_backlight once crbug.com/950346 support hardware dependency.
		SoftwareDeps: []string{"no_qemu", "chrome", "display_backlight"},
		Data:         []string{"screenshot1_reference.png", "screenshot2_reference.png", "windowmanagertest.html"},
	})
}

func Sanity(ctx context.Context, s *testing.State) {
	number, err := graphics.NumberOfOutputsConnected(ctx)
	if err != nil {
		s.Fatal("Failed to get current connected monitors: ", err)
	}

	// TODO(pwang): Switch to use hardware dependency once it is ready.
	if number <= 0 {
		s.Fatal("Skipped as no monitor is detected")
	}

	// Explicitly switching to GUI. If the display is sleeping, this turns on it.
	if err := switchToGUI(ctx); err != nil {
		s.Fatal("Failed to switch to GUI: ", err)
	}
	testSomethingOnScreen(ctx, s)
	testGeneratedScreenshot(ctx, s)
}

func switchToGUI(ctx context.Context) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return err
	}
	defer kb.Close()
	return kb.Accel(ctx, "Ctrl+Alt+F1")
}

// testSomethingOnScreen makes sure something is drawn on the screen, i.e. the display is
// not completely black.
func testSomethingOnScreen(ctx context.Context, s *testing.State) {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	// Wait until screenshot can be taken.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		f, err := ioutil.TempFile("", "screenshot_test-*.png")
		if err != nil {
			return testing.PollBreak(err)
		}
		if err := f.Close(); err != nil {
			return testing.PollBreak(err)
		}
		defer os.Remove(f.Name())
		if err := screenshot.Capture(ctx, f.Name()); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		s.Error("Screen didn't get ready: ", err)
		return
	}

	signinPng := filepath.Join(s.OutDir(), "signin.png")
	if err := screenshot.Capture(ctx, signinPng); err != nil {
		s.Error("Failed to take screenshot on signin page: ", err)
		return
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Error("Failed to log into Chrome: ", err)
		return
	}
	defer cr.Close(ctx)

	conn, err := cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Error("Failed to open chrome://settings: ", err)
		return
	}
	defer conn.Close()

	settingsPng := filepath.Join(s.OutDir(), "settings.png")
	if err := screenshot.Capture(ctx, settingsPng); err != nil {
		s.Error("Failed to take screenshot on settings page: ", err)
		return
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Error("Failed to take TestAPI connection: ", err)
		return
	}

	info, err := display.GetInfo(ctx, tconn)
	if err != nil || len(info) < 1 {
		s.Error("Failed to get display info: ", err)
		return
	}

	w := int64(info[0].Bounds.Width)
	h := int64(info[0].Bounds.Height)
	// The threshold of the file size heuristically determined.
	// Larger size means "some more information" on the screen. Smaller size
	// means the screenshot is "empty" (i.e. close to solid color).
	threshold := 15 * (w * h) / 1000000

	for _, png := range []string{signinPng, settingsPng} {
		if info, err := os.Stat(png); err != nil {
			s.Errorf("Failed to stat %s: %v", filepath.Base(png), err)
		} else if size := info.Size(); size < threshold {
			// Screenshot filesize is smaller than expected. This indicates
			// that there is nothing on screen. This ChromeOS image
			// could be unusable.
			s.Errorf("Screenshot file %s is too small: got %d, want >= %d", filepath.Base(png), size, threshold)
		}
	}
}

// testGeneratedScreenshot draws a texture with a soft ellipse twice and captures each image.
// Compares the output fuzzily against the reference images.
func testGeneratedScreenshot(ctx context.Context, s *testing.State) {
	// We are using Chrome+WebGL to draw and verify two ellipses.  For
	// sanity test, we don't care about using hw overlays in conjunction
	// with screenshots, as that could introduce unrelated problems.  So
	// we disable hw overlays for this subtest:
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-hardware-overlays=\"\""))
	if err != nil {
		s.Error("Failed to log into Chrome: ", err)
		return
	}
	defer cr.Close(ctx)

	// As we are starting chrome with non-standard args, we should cleanup in the end:
	defer upstart.RestartJob(ctx, "ui")

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Error("failed to initialize the keyboard writer", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	tempdir, err := ioutil.TempDir("", "generated_screenshot")
	if err != nil {
		s.Error("Failed to create a tempdir: ", err)
		return
	}
	defer os.RemoveAll(tempdir)

	url := path.Join(server.URL, "windowmanagertest.html")
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Error("Failed to open %v: ", url, err)
		return
	}
	defer conn.Close()

	// Make the webgl go to full screen mode by pressing 'f': requestFullScreen() needs a user gesture.
	if err := ew.Type(ctx, "f"); err != nil {
		s.Error("failed to inject the 'f' key")
	}

	// Delay to wait for the exit-fullscreen prompt to go away:
	if err := testing.Sleep(ctx, 5 * time.Second); err != nil {
		s.Fatal("Cannot sleep: ", err)
		return
	}

	resizePng := func(src, dst string, width, height int) error {
		return testexec.CommandContext(ctx, "convert", "-channel", "RGB", "-colorspace", "RGB", "-depth", "8", "-resize", fmt.Sprintf("%dx%d!", width, height), src, dst).Run(testexec.DumpLogOnError)
	}

	testScreenshotResult := func(name string) error {
		// Give the javascript time to react to the input and draw the pattern:
		if err := testing.Sleep(ctx, 1 * time.Second); err != nil {
			s.Fatal("Cannot sleep: ", err)
		}

		generated := filepath.Join(tempdir, name + "_generated.png")
		if err := screenshot.Capture(ctx, generated); err != nil {
			s.Error("Failed to take screenshot ", name, ": ", err)
			return err
		}

		resized := filepath.Join(s.OutDir(), name + "_resized.png")
		if err := resizePng(generated, resized, 100, 100); err != nil {
			s.Error("Failed to resize ", name, ": ", err)
			return err
		}

		reference := s.DataPath(name + "_reference.png")
		if err := testexec.CommandContext(ctx, "perceptualdiff", "-verbose", reference, resized).Run(testexec.DumpLogOnError); err != nil {
			s.Error("Unexpected diff from reference for ", name, ": ", err)
			return err
		}

		return nil
	}

	testOneScreenshot := func(name string, key string) error {

		// Trigger draw of white ellipse:
		if err := ew.Type(ctx, key); err != nil {
			s.Error("failed to inject the '%s' key", key)
			return err
		}

		// Normally we should only need a single try, but to be resilient
		// against slowdowns, network issues, etc, try a few times:
		for n := 1; ; n++ {
			err := testScreenshotResult(name)
			if err != nil {
				s.Logf("%s failed on the %dth iteration", name, n)
				if n == 5 {
					s.Error("Giving up on ", name)
					return err;
				}
			} else {
				break
			}
		}

		return nil
	}

	if err := testOneScreenshot("screenshot1", "w"); err != nil {
		return
	}
	if err := testOneScreenshot("screenshot2", "b"); err != nil {
		return
	}
}
