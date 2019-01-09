// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeActivity,
		Desc:         "Verifies that resizing ARC++ applications work",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "android_p", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

type activity struct {
	ctx          context.Context
	a            *arc.ARC
	pkgName      string
	activityName string
}

type rect struct {
	left, top, right, bottom int
}

func newActivity(ctx context.Context, a *arc.ARC, pkgName string, activityName string) (*activity, error) {
	return &activity{ctx, a, pkgName, activityName}, nil
}

func (ac *activity) start() error {
	cmd := ac.a.Command(ac.ctx, "am", "start", "-W", ac.pkgName+"/"+ac.activityName)
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	// "adb shell" doesn't distinguish between a failed/successful run. For that we have to parse the output.
	re := regexp.MustCompile("(?m)^Error:")
	if re.MatchString(string(output)) {
		testing.ContextLog(ac.ctx, "Failed to start activity: ", string(output))
		return errors.New("failed to start activity")
	}
	return nil
}

func (ac *activity) stop() error {
	// "adb shell am force-stop" has no output. So the error from Run() is returned.
	return ac.a.Command(ac.ctx, "am", "force-stop", ac.pkgName).Run()
}

func (ac *activity) bounds() (rect, error) {
	cmd := ac.a.Command(ac.ctx, "dumpsys", "window", "displays")
	output, err := cmd.Output()
	if err != nil {
		return rect{}, errors.Wrap(err, "failed to launch dumpsys")
	}

	// Line that we are interested in parsing:
	//  mBounds=[0,0][2400,1600]
	//  mdr=false
	//  appTokens=[AppWindowToken{85a61b token=Token{42ff82a ActivityRecord{e8d1d15 u0 org.chromium.arc.home/.HomeActivity t2}}}]
	// We are interested in "mBounds="
	regStr := `\s*mBounds=\[([0-9]*),([0-9]*)\]\[([0-9]*),([0-9]*)\]\n\s*mdr=.*\n\s*appTokens=.*` + ac.pkgName + "/" + ac.activityName + `.*\n`
	re := regexp.MustCompile(regStr)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 5 {
		return rect{}, errors.New("failed to parse dumpsys output; activity not running perhaps?")
	}
	// left, top, right, bottom
	var bounds [4]int
	for i := 0; i < 4; i++ {
		bounds[i], err = strconv.Atoi(groups[i+1])
		if err != nil {
			return rect{}, errors.Wrap(err, "could not parse bounds")
		}
	}
	return rect{bounds[0], bounds[1], bounds[2], bounds[3]}, nil
}

func (ac *activity) resize() {
}

func ResizeActivity(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	ac, _ := newActivity(ctx, a, "com.android.settings", ".Settings")
	if err := ac.start(); err != nil {
		s.Fatal("Could not launch settings: ", err)
	}

	rect, err := ac.bounds()
	if err != nil {
		s.Fatal("Error getting bounds: ", err)
	}
	s.Logf("Bounds = %v", rect)

	screenshotName := "screenshot.png"
	path := filepath.Join(s.OutDir(), screenshotName)
	s.Logf("Screenshot should be placed: %s\n", path)

	if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
		s.Fatal("Error taking screenshot: ", err)
	}

	s.Log("Sleeping for 10 seconds...")
	sleep(ctx, 10*time.Second)
}

func sleep(ctx context.Context, t time.Duration) error {
	select {
	case <-time.After(t):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
