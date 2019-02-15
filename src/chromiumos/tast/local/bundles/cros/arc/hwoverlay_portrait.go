// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// rotation represents the rotation angles: 0, 90, 180 or 270.
type rotation int

const (
	// rotation0 represents a rotation of 0 degrees.
	rotation0 rotation = iota
	// rotation90 represents a rotation of 90 degrees.
	rotation90
	// rotation represents a rotation of 180 degrees.
	rotation180
	// rotation270 represents a rotation of 270 degrees.
	rotation270
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HWOverlayPortrait,
		Desc:     "Checks that hardware overlay works with ARC applications",
		Contacts: []string{"ricardoq@chromium.org", "arc-eng@google.com"},
		// TODO(ricardoq): enable test once the the bug that fixes hardware overlay gets fixed. See: http://b/120557146
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"hw_overlay", "tablet_mode", "android", "android_p", "chrome_login"},
		Timeout:      3 * time.Minute,
	})
}

func HWOverlayPortrait(ctx context.Context, s *testing.State) {
	// Parses /sys/kernel/debug/dri/?/state and compares the different CRTC buffer sizes
	// with the activity window frame size. If there is a match, it means that HW overlay is being used.
	// See https://crbug.com/932778 for other alternatives.

	// TODO(ricardoq): Add clamshell mode tests.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Any ARC++ activity could be used for this test. Using one that is already installed.
	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	deviceID, err := internalDisplayID(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get internal display ID: ", err)
	}

	stateFile, err := driStateFile()
	// Should not fail, since it is guaranteed by "hw_overlay" SoftwareDeps.
	if err != nil {
		s.Fatal("Hardware overlay not supported. Perhaps 'hw_overlay' USE property added to the incorrect board?: ", err)
	}

	// Leave Chromebook in reasonable state.
	defer func() { setRotation(ctx, cr, deviceID, rotation0) }()

	for _, entry := range []struct {
		rot  rotation
		desc string
	}{
		{rotation0, "0"},
		{rotation90, "90"},
		{rotation180, "180"},
		{rotation270, "270"},
	} {
		s.Log("Setting display rotation to ", entry.desc)

		if err := setRotation(ctx, cr, deviceID, entry.rot); err != nil {
			s.Error("Failed to set rotation: ", err)
		}

		// While rotating, HW overlay is disabled. It might take a few seconds to get active again.
		err := testing.Poll(ctx, func(ctx context.Context) error {
			return verifyHWOverlay(ctx, act, stateFile)
		}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Second})
		if err != nil {
			s.Errorf("verifyHWOverlay failed for rotation %s: %s", entry.desc, err)
		}
	}
}

// verifyHWOverlay verifies that hardware overlay is being used by comparing the window frame size
// with the different CRTC buffer sizes.
// path is the fullpath to the DRI state file.
func verifyHWOverlay(ctx context.Context, a *arc.Activity, path string) error {
	frame, err := a.WindowFrame(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get activity window frame")
	}
	w := frame.Right - frame.Left
	h := frame.Bottom - frame.Top

	// Rare case when frame is empty while rotating.
	if w == 0 && h == 0 {
		return errors.New("invalid window frame size: 0x0")
	}

	dat, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "error reading file: "+path)
	}

	// A tipical CRTC entry looks like the following:
	// plane[27]: plane 1A
	//     crtc=(null)
	//     fb=0
	//     crtc-pos=3000x2000+0+0
	//     src-pos=3000.000000x2000.000000+0.000000+0.000000
	//     rotation=1

	// A non-zero value in crtc-pos indicates that a CRTC buffer is being used.
	// The format of crtc-pos is size + offset. If there is a CRTC buffer with the same size as the
	// activity frame, we confirm that HW overlay is being used for ARC++.
	regStr := `(?m)` + // Enable multiline.
		`crtc-pos=(\d+)x(\d+)\+(\d+)\+(\d+)` // Match CRTC size + offset

	re := regexp.MustCompile(regStr)
	l := re.FindAllStringSubmatch(string(dat), -1)

	for _, groups := range l {
		if len(groups) != 5 {
			return errors.New("invalid regexp match")
		}

		// For the comparison, CRTC buffer size is good enough. Ignoring the CRTC offset.
		var crtcW, crtcH int
		for i, dst := range map[int]*int{1: &crtcW, 2: &crtcH} {
			*dst, err = strconv.Atoi(groups[i])
			if err != nil {
				return errors.Wrap(err, "could not parse CRTC bounds")
			}
		}

		// CRTC size is always in native size (non-rotated). Frame size could be rotated or not.
		if (crtcW == w && crtcH == h) || (crtcW == h && crtcH == w) {
			return nil
		}
	}
	return errors.Errorf("could not find CRTC buffer of size: %d,%d", w, h)
}

// driStateFile returns the path to the DRI state file, which is the file that contains the hardware overlay information.
// Returns error if file cannot be found.
func driStateFile() (driFile string, err error) {
	// The 'state' file is the one that has the HW overlay state. Depending on the device, it
	// could be either in the .../dri/0/ or .../dri/1/ directories.
	driStateFiles := []string{"/sys/kernel/debug/dri/0/state", "/sys/kernel/debug/dri/1/state"}
	for i := 0; i < len(driStateFiles); i++ {
		_, err := os.Stat(driStateFiles[i])
		if err == nil {
			return driStateFiles[i], nil
		}
	}
	return "", errors.New("dri state file not found")
}

// internalDisplayID returns the display ID of the internal display.
func internalDisplayID(ctx context.Context, cr *chrome.Chrome) (id string, err error) {
	displays, err := displaysInfo(ctx, cr)
	if err != nil {
		return "", errors.Wrap(err, "failed to get displays info")
	}

	for _, d := range displays {
		val, ok := d["isInternal"]
		if !ok {
			return "", errors.New("could not find 'isInternal' property")
		}

		isInternal := val.(bool)
		if !isInternal {
			continue
		}

		val, ok = d["id"]
		if !ok {
			return "", errors.New("could not find 'id' property")
		}
		return val.(string), nil
	}
	return "", errors.New("could not found internal id")
}

// displaysInfo requests the information for all attached display devices.
// info is the value returned from JS API: chrome.system.display.getInfo()
// See: https://developer.chrome.com/apps/system_display#method-getInfo
func displaysInfo(ctx context.Context, cr *chrome.Chrome) (info []map[string]interface{}, err error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	if err = tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
			chrome.system.display.getInfo({}, (info) => {
			  if (chrome.runtime.lastError === undefined) {
				resolve(info);
			  } else {
				reject(chrome.runtime.lastError.message);
			  }
			});
		  })`, &info); err != nil {
		return nil, err
	}
	return info, nil
}

// setRotation sets the rotation for the display specified by id.
// The rotation is set clockwise. r is the new rotation angle.
func setRotation(ctx context.Context, cr *chrome.Chrome, id string, r rotation) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	var rot string
	switch r {
	case rotation0:
		rot = "0"
	case rotation90:
		rot = "90"
	case rotation180:
		rot = "180"
	case rotation270:
		rot = "270"
	}

	if err = tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
			chrome.system.display.setDisplayProperties("`+id+`", {"rotation":`+rot+`}, () => {
			  if (chrome.runtime.lastError === undefined) {
				resolve();
			  } else {
				reject(chrome.runtime.lastError.message);
			  }
			});
		  })`, nil); err != nil {
		return err
	}
	return nil
}
