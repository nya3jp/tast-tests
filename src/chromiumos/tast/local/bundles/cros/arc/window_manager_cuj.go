// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

const (
	// Apk compiled against target Sdk = 23
	wmPkg23 = "org.chromium.arc.testapp.windowmanager23"
	// Apk compiled against target Sdk >= 24
	wmPkg24 = "org.chromium.arc.testapp.windowmanager24"

	wmResizeableLandscapeActivity      = "org.chromium.arc.testapp.windowmanager.ResizeableLandscapeActivity"
	wmNonResizeableLandscapeActivity   = "org.chromium.arc.testapp.windowmanager.NonResizeableLandscapeActivity"
	wmResizeableUnspecifiedActivity    = "org.chromium.arc.testapp.windowmanager.ResizeableUnspecifiedActivity"
	wmNonResizeableUnspecifiedActivity = "org.chromium.arc.testapp.windowmanager.NonResizeableUnspecifiedActivity"
	wmResizeablePortraitActivity       = "org.chromium.arc.testapp.windowmanager.ResizeablePortraitActivity"
	wmNonResizeablePortraitActivity    = "org.chromium.arc.testapp.windowmanager.NonResizeablePortraitActivity"

	wmBack     = "back"
	wmMinimize = "minimize"
	wmMaximize = "maximize"
	wmRestore  = "restore"
	wmClose    = "close"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowManagerCUJ,
		Desc:         "Verifies that Window Manager Critical User Journey behaves as described in go/arc-wm-p",
		Contacts:     []string{"ricardoq@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcWMTestApp_24.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

func WindowManagerCUJ(ctx context.Context, s *testing.State) {
	const (
		apk23 = "ArcWMTestApp_23.apk"
		apk24 = "ArcWMTestApp_24.apk"
	)

	a := s.PreValue().(arc.PreData).ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Starting app")

	if err := a.Install(ctx, s.DataPath(apk24)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := wmCUJ1(ctx, a, d); err != nil {
		s.Fatal("Failed to test Slide #6: ", err)
	}
}

// wmCUJ1 verifies the WM Critical User Journey (CUJ) case #1, as defined in:
// go/arc-wm-p "Clamshell: default launch behavior - Android NYC or above" (slide #6).
func wmCUJ1(ctx context.Context, a *arc.ARC, d *ui.Device) error {
	wmCaptionBMRC := []string{wmBack, wmMinimize, wmRestore, wmClose}
	wmCaptionBMMC := []string{wmBack, wmMinimize, wmMaximize, wmClose}
	wmCaptionBMC := []string{wmBack, wmMinimize, wmClose}
	// Reset WM state to default values.
	if err := a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate").Run(); err != nil {
		return err
	}
	for _, test := range []struct {
		name          string
		act           string
		wantedState   arc.WindowState
		wantedCaption []string
	}{
		// Use Cases taken from go/arc-wm-p slide #6.
		// Defined as Window #A.
		{"Madre1", wmResizeableLandscapeActivity, arc.WindowStateMaximized, wmCaptionBMRC},
		// Defined as Window #B.
		{"Madre2", wmNonResizeableLandscapeActivity, arc.WindowStateMaximized, wmCaptionBMC},
		// Defined as Window #A.
		{"Madre5", wmResizeableUnspecifiedActivity, arc.WindowStateMaximized, wmCaptionBMRC},
		// Defined as Window #B.
		{"Madre6", wmNonResizeableUnspecifiedActivity, arc.WindowStateMaximized, wmCaptionBMC},
		// Defined as Window #C.
		{"Madre3", wmResizeablePortraitActivity, arc.WindowStateNormal, wmCaptionBMMC},
		// Defined as Window #D.
		// TODO(ricardoq): detect that wanted state is maximized + pillarbox mode.
		{"Madre4", wmNonResizeablePortraitActivity, arc.WindowStateMaximized, wmCaptionBMC},
	} {
		testing.ContextLog(ctx, "Testing case ", test.name)
		act, err := arc.NewActivity(a, wmPkg24, test.act)
		if err != nil {
			return err
		}
		defer act.Close()

		if err := act.Start(ctx); err != nil {
			return err
		}
		state, err := act.GetWindowState(ctx)
		if err != nil {
			return err
		}
		if state != test.wantedState {
			return errors.Errorf("invalid window state %v, want %v", state, test.wantedState)
		}

		b, err := wmCaptionButtons(ctx, d)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(b, test.wantedCaption) {
			return errors.Errorf("invalid captions buttons %+v, want %+v", b, test.wantedCaption)
		}
		if err := act.Stop(ctx); err != nil {
			return errors.Wrapf(err, "could not stop activity %v", test.act)
		}
	}
	return nil
}

// wmCaptionButtons returns the caption buttons that are present in the window.
func wmCaptionButtons(ctx context.Context, d *ui.Device) (buttons []string, err error) {
	s, err := getWmState(ctx, d)
	if err != nil {
		return nil, err
	}
	return s.Buttons, nil
}

// wmState represents the state of ArcWMTestApp activity.
type wmState struct {
	WindowState       string      `json:"windowState"`
	Orientation       string      `json:"orientation"`
	DeviceMode        string      `json:"deviceMode"`
	ActivityNr        int         `json:"activityNr"`
	CaptionVisibility string      `json:"captionVisibility"`
	Zoomed            bool        `json:"zoomed"`
	Rotation          int         `json:"rotation"`
	Buttons           []string    `json:"buttons"`
	Accel             interface{} `json:"accel"`
}

// getWmState returns the state from the ArcWMTest activity.
// The state is taken by parsing the activity's TextView which contains the state in JSON format.
func getWmState(ctx context.Context, d *ui.Device) (*wmState, error) {
	obj := d.Object(ui.ClassName("android.widget.TextView"), ui.ResourceIDMatches(".+?(/caption_text_view)$"))
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		return nil, err
	}
	s, err := obj.GetText(ctx)
	if err != nil {
		return nil, err
	}
	var state wmState
	if err := json.Unmarshal([]byte(s), &state); err != nil {
		return nil, errors.Wrap(err, "failed unmarshaling state")
	}
	return &state, nil
}
