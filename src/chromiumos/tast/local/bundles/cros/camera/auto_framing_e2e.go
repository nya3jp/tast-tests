// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/testpage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoFramingE2E,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Camera Auto-framing end-to-end integration test",
		Contacts:     []string{"kamesan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"camera_feature_auto_framing", "chrome", caps.BuiltinCamera},
		Data:         []string{"camera_page.html", "camera_page.js"},
	})
}

func AutoFramingE2E(ctx context.Context, s *testing.State) {
	// AutozoomState values should match AutoFramingStreamManipulator::State enum in the camera service.
	type AutozoomState int

	const (
		Off             AutozoomState = 0
		TransitionToOn                = 1
		On                            = 2
		TransitionToOff               = 3
	)

	cr, err := chrome.New(ctx, chrome.ExtraArgs(
		"--auto-framing-override=force-enabled",
		// Avoid the need to grant camera/microphone permissions.
		"--use-fake-ui-for-media-stream",
	))
	if err != nil {
		s.Error("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	page := testpage.New(server.URL)
	cst := testpage.NewConstraints(640, 480, testpage.UserFacing)
	if err := page.OpenWithConstraints(ctx, cr, cst); err != nil {
		s.Fatal("Failed to open camera test page: ", err)
	}

	sysLogReader, err := syslog.NewLineReader(ctx, syslog.MessageFile /*fromStart=*/, false /*opts=*/, nil)
	if err != nil {
		s.Fatal("Failed to start log reader: ", err)
	}
	defer sysLogReader.Close()

	readAutozoomState := func() (AutozoomState, error) {
		autozoomStateChangeMessagePattern := regexp.MustCompile(
			`cros_camera_service\[[0-9]+\]: StateTransitionOnThread\(\): State: [0-3] -> ([0-3])`)
		state := Off
		isRead := false
		for {
			line, err := sysLogReader.ReadLine()
			if err != nil {
				break
			}
			match := autozoomStateChangeMessagePattern.FindStringSubmatch(line)
			if len(match) != 2 {
				continue
			}
			if s, err := strconv.Atoi(match[1]); err == nil {
				isRead = true
				state = AutozoomState(s)
			}
		}
		if !isRead {
			return Off, errors.New("Autozoom state not found in syslog")
		}
		return state, nil
	}

	if err := quicksettings.Expand(ctx, tconn); err != nil {
		s.Fatal("Failed to expand Quick Settings: ", err)
	}

	ui := uiauto.New(tconn)

	// Check the Autozoom button state matches camera service.
	const stateChangeDuration = time.Second
	testActions := []struct {
		clickTimes    int
		expectedState AutozoomState
	}{
		{1, On}, {1, Off}, {2, Off}, {3, On}, {6, On}, {9, Off},
	}
	for _, t := range testActions {
		for i := 0; i < t.clickTimes; i++ {
			if err := ui.LeftClick(quicksettings.PodIconButton(quicksettings.SettingPodAutozoom))(ctx); err != nil {
				s.Fatal("Failed to click Autozoom feature pod icon button: ", err)
			}
		}
		testing.Sleep(ctx, stateChangeDuration)
		if state, err := readAutozoomState(); err != nil {
			s.Error("Failed to read Autozoom state: ", err)
		} else if state != t.expectedState {
			s.Errorf("Expected Autozoom state %v but read %v", t.expectedState, state)
		}
	}
}
