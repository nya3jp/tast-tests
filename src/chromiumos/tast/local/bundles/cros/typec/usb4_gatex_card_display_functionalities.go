// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/typec/setup"
	typecutilshelper "chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         USB4GatexCardDisplayFunctionalities,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies Display functionalities on USB4 Gatkex card connected to USB4 port using 40G passive cable after hot plug/unplug",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:typec"},
		Data:         []string{"testcert.p12", "bear-320x240.h264.mp4", "video.html", "test_config.json", "playback.js"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey", "typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIP"},
		Fixture:      "chromeLoggedInThunderbolt",
		HardwareDeps: hwdep.D(setup.ThunderboltSupportedDevices()),
		Timeout:      7 * time.Minute,
	})
}

func USB4GatexCardDisplayFunctionalities(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	// Config file which contains expected values of USB4 parameters.
	const jsonTestConfig = "test_config.json"

	// TBT port ID in the DUT.
	dutPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	if err := typecutils.EnablePeripheralDataAccess(ctx, s.DataPath("testcert.p12")); err != nil {
		s.Fatal("Failed to enable peripheral data access setting: ", err)
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(s, "Failed to create test API connection: ", err)
	}

	// Read json config file.
	jsonData, err := ioutil.ReadFile(s.DataPath(jsonTestConfig))
	if err != nil {
		s.Fatal("Failed to read response data: ", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// Checking for USB4 config data.
	usb4Val, ok := data["USB4"].(map[string]interface{})
	if !ok {
		s.Fatal("Failed to found USB4 config data in JSON file")
	}

	// Create C-Switch session that performs hot plug-unplug on USB4 device.
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

	if err := typecutilshelper.CheckUSBPdMuxinfo(ctx, "USB4=1"); err != nil {
		s.Fatal("Failed to verify dmesg logs: ", err)
	}

	if _, err := cswitch.IsDeviceEnumerated(ctx, usb4Val["device_name"].(string), dutPort); err != nil {
		s.Fatal("Failed to enumerate USB4 device: ", err)
	}

	if err := typecutilshelper.FindConnectedDisplay(ctx, 1); err != nil {
		s.Fatal("Failed to find connected display: ", err)
	}

	isTypecHDMI := false
	if err := typecutilshelper.CheckDisplayInfo(ctx, isTypecHDMI); err != nil {
		s.Fatal("Failed to check display info : ", err)
	}

	// Set mirror mode display.
	isMirrorSet := true
	if err := typecutilshelper.SetMirrorDisplay(ctx, tconn, isMirrorSet); err != nil {
		s.Fatal("Failed to set mirror mode: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()
	url := srv.URL + "/video.html"
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to load video.html: ", err)
	}

	videoFile := "bear-320x240.h264.mp4"
	if err := conn.Call(ctx, nil, "playRepeatedly", videoFile); err != nil {
		s.Fatal("Failed to play video: ", err)
	}
	defer conn.Close()
}
