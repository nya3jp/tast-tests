// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"unsafe"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMediaTrackAdvancedControls,
		Desc:         "Opens CCA and verifies the MediaTrack advanced controls",
		Contacts:     []string{"mojahsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIMediaTrackAdvancedControls(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Can't get number of cameras: ", err)
	}

	if numCameras > 1 {
		verifyAdvancedControls(ctx, s, app)
		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switch camera failed: ", err)
		}
		verifyAdvancedControls(ctx, s, app)
	} else if numCameras == 1 {
		verifyAdvancedControls(ctx, s, app)
	} else {
		s.Fatal("No camera found")
	}
}

func verifyAdvancedControls(ctx context.Context, s *testing.State, app *cca.App) {
	pairControls := [...][2]string{{"exposureMode", "exposureTime"}, {"focusMode", "focusDistance"}, {"whiteBalanceMode", "colorTemperature"}}
	controls := [...]string{
		"exposureCompensation",
		"iso",
		"brightness",
		"contrast",
		"saturation",
		"sharpness",
		"pan",
		"tilt",
		"zoom"}
	capabilities, err := app.GetMediaTrackCapabilities(ctx)
	if err != nil {
		s.Fatal("Can't get capabilities: ", err)
		return
	}
	s.Log(capabilities)
	for _, pairControl := range pairControls {
		ss := reflect.ValueOf(&capabilities).Elem().FieldByName(strings.Title(pairControl[0]))
		if ss.IsNil() {
			continue
		}
		s.Log("support " + pairControl[0])
		mode := *(*[]string)(unsafe.Pointer(ss.Pointer()))
		s.Log(mode)
		sss := reflect.ValueOf(&capabilities).Elem().FieldByName(strings.Title(pairControl[1]))
		if sss.IsNil() {
			s.Fatal("Pair control is not supported: " + strings.Title(pairControl[1]))
			return
		}
		settingRange := *(*cca.MediaSettingsRange)(unsafe.Pointer(sss.Pointer()))
		s.Log(settingRange)
	}

	for _, control := range controls {
		ss := reflect.ValueOf(&capabilities).Elem().FieldByName(strings.Title(control))
		if ss.IsNil() {
			continue
		}
		settingRange := *(*cca.MediaSettingsRange)(unsafe.Pointer(ss.Pointer()))
		s.Log("support " + control)
		verifyControlRange(ctx, s, app, control, settingRange)
	}
}

func verifyControlRange(ctx context.Context, s *testing.State, app *cca.App, control string, settingRange cca.MediaSettingsRange) {
	for i := settingRange.Min; i <= settingRange.Max; i += settingRange.Step {
		constraint := fmt.Sprintf("{\"advanced\": [{\"%s\": %f}]}", control, i)
		s.Log(constraint)
		app.ApplyMediaTrackConstraints(ctx, constraint)
		settings, err := app.GetMediaTrackSettings(ctx)
		if err != nil {
			s.Fatal("Can't get settings: ", err)
			return
		}
		value := *(*float32)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(control)).Pointer()))
		log := fmt.Sprintf("Set %s to %f, get %f", control, i, value)
		s.Log(log)
	}
}
