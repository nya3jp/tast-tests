// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/testing"
)

type displayFunctionalities struct {
	isTypecHDMI bool
	isTypecDP   bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExternalDisplayPlugUnplugVideoPlayback,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies HDMI plug/unplug using USB type-C adapter during audio/video playback",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:typec"},
		Data:         []string{"bear-320x240.h264.mp4", "video.html", "playback.js"},
		Vars:         []string{"typec.cSwitchPort", "typec.domainIP"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "hdmi",
			Val: displayFunctionalities{
				isTypecHDMI: true,
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "dp",
			Val: displayFunctionalities{
				isTypecDP: true,
			},
			Timeout: 5 * time.Minute,
		}},
	})
}

func ExternalDisplayPlugUnplugVideoPlayback(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// Test Params.
	testParms := s.Param().(displayFunctionalities)

	// Config file which contains expected values of USB4 parameters.
	const jsonTestConfig = "test_config.json"

	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard input: ", err)
	}

	// Create C-Switch session that performs hot plug-unplug on TBT device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create session: ", err)
	}

	cSwitchOFF := "0"
	defer func(ctx context.Context) {
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}
		if err := cswitch.CloseSession(ctx, sessionID, domainIP); err != nil {
			s.Log("Failed to close session: ", err)
		}
	}(cleanupCtx)

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	if err := typecutils.FindConnectedDisplay(ctx, 1); err != nil {
		s.Fatal("Failed to find connected display: ", err)
	}

	if err := typecutils.CheckDisplayInfo(ctx, testParms.isTypecHDMI, testParms.isTypecDP); err != nil {
		s.Fatal("Failed to check display info : ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()
	url := srv.URL + "/video.html"
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to load video_with_rounded_corners.html: ", err)
	}

	display := typecutils.SwitchWindowToDisplay(ctx, tconn, kb, true)
	if err := display(ctx); err != nil {
		s.Fatal("Failed to switch windows to display: ", err)
	}

	videoFile := "bear-320x240.h264.mp4"
	if err := conn.Call(ctx, nil, "playUntilEnd", videoFile, true); err != nil {
		s.Fatal("Failed to play video: ", err)
	}

	expectedAudioOuputNode := "HDMI"
	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	// Get current audio output device info.
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Fatal("Failed to get the selected audio device: ", err)
	}

	if deviceType != expectedAudioOuputNode {
		if err := cras.SetActiveNodeByType(ctx, expectedAudioOuputNode); err != nil {
			s.Fatalf("Failed to select active device %s: %v", expectedAudioOuputNode, err)
		}
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			s.Fatal("Failed to get the selected audio device: ", err)
		}
		if deviceType != expectedAudioOuputNode {
			s.Fatalf("Failed to set the audio node type: got %q; want %q", deviceType, expectedAudioOuputNode)
		}
	}
	out, err := testexec.CommandContext(ctx, "cras_test_client").Output()
	if err != nil {
		s.Fatal("Failed to exceute cras_test_client command: ", err)
	}
	re := regexp.MustCompile(`yes.*HDMI.*2\*`)
	if !re.MatchString(string(out)) {
		s.Fatal("Failed to select HDMI as output audio node")
	}

	if err := typecutils.VerifyAudioRoute(ctx, deviceName); err != nil {
		s.Fatalf("Failed to verify audio routing through %q: %v", expectedAudioOuputNode, err)
	}

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}
}
