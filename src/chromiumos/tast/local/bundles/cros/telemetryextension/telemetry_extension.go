// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	manifestJSON = "manifest.json"
	optionsHTML  = "options.html"
	swJS         = "sw.js"
)

var dataFiles = []string{manifestJSON, optionsHTML, swJS}

func init() {
	testing.AddTest(&testing.Test{
		Func: TelemetryExtension,
		Desc: "Tests TelemetryExtension core functionality such as API, permissions, communication with PWA",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         dataFiles,
		Fixture:      "chromeLoggedIn",
	})
}

// TelemetryExtension tests that Telemetry Extension has access to APIs and can talk with PWA.
func TelemetryExtension(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	// Load PWA page first to do not switch between tabs after loading extension.
	conn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create connection to google.com: ", err)
	}
	defer conn.Close()

	if err := chrome.AddTastLibrary(ctx, conn); err != nil {
		s.Fatal("Failed to add Tast library to PWA: ", err)
	}

	dir := "/home/chronos/user/MyFiles/Downloads/telemetry-extension"
	if err := os.Mkdir(dir, 0777); err != nil {
		s.Fatal("Failed to create temporary directory for TelemetryExtension: ", err)
	}
	defer os.RemoveAll(dir)

	if err := os.Chown(dir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		s.Fatal("Failed to chown TelemetryExtension dir: ", err)
	}

	for _, file := range dataFiles {
		if err := fsutil.CopyFile(s.DataPath(file), filepath.Join(dir, file)); err != nil {
			s.Fatalf("Failed to copy %q file to %q: %v", file, dir, err)
		}

		if err := os.Chown(filepath.Join(dir, file), int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
			s.Fatalf("Failed to chown %q: %v", file, err)
		}
	}

	// TODO: remove extension from Chrome at the end.
	extensionsConn, err := cr.NewConn(ctx, "chrome://extensions")
	if err != nil {
		s.Fatal("Failed to create connection to Chrome extensions page: ", err)
	}
	defer extensionsConn.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connections: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)
	developerMode := nodewith.Name("Developer mode").Role(role.ToggleButton)
	loadUnpacked := nodewith.Name("Load unpacked").Role(role.Button)
	telemetryDir := nodewith.Name("telemetry-extension").Role(role.ListBoxOption)
	openButton := nodewith.Name("Open").Role(role.Button)
	testTelemetryExtension := nodewith.Name("Test Telemetry Extension").Role(role.Heading)
	details := nodewith.Name("Details").Role(role.Button).Nth(1)
	if err := uiauto.Combine("allow serial number permission",
		ui.WithTimeout(10*time.Second).WaitUntilExists(developerMode),
		ui.LeftClick(developerMode),
		ui.WithTimeout(10*time.Second).WaitUntilExists(loadUnpacked),
		ui.LeftClick(loadUnpacked),
		ui.WithTimeout(15*time.Second).WaitUntilExists(telemetryDir),
		ui.LeftClick(telemetryDir),
		ui.WithTimeout(10*time.Second).WaitUntilExists(openButton),
		ui.LeftClick(openButton),
		ui.WithTimeout(5*time.Second).WaitUntilExists(testTelemetryExtension),
		ui.LeftClick(details),
	)(ctx); err != nil {
		s.Fatal("Failed to load Telemetry extension: ", err)
	}

	var resp swResponse
	if err := conn.Call(ctx, &resp,
		"tast.promisify(chrome.runtime.sendMessage)",
		"gogonhoemckpdpadfnjnpgbjpbjnodgc",
		"ping message",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	wantResp, err := expectedSwResponse(ctx)
	if err != nil {
		s.Fatal("Failed to get expected response: ", err)
	}

	// These fields should be empty due to lack of "os.telemetry.serial_number" permission.
	if diff := cmp.Diff(wantResp, resp,
		cmpopts.IgnoreFields(swResponse{}, "OemData"),
		cmpopts.IgnoreFields(vpdInfo{}, "SerialNumber"),
	); diff != "" {
		s.Fatal("Unexpected response from Telemetry extension (-want +got): ", diff)
	}

	optionsButton := nodewith.Name("Extension options").Role(role.Link)
	requestButton := nodewith.Name("Request serial number permission").Role(role.Button)
	allowButton := nodewith.Name("Allow").Role(role.Button)
	if err := uiauto.Combine("allow serial number permission",
		ui.WithTimeout(5*time.Second).WaitUntilExists(optionsButton),
		ui.FocusAndWait(optionsButton),
		ui.LeftClick(optionsButton),
		ui.WithTimeout(5*time.Second).WaitUntilExists(requestButton),
		ui.LeftClick(requestButton),
		ui.WithTimeout(5*time.Second).WaitUntilExists(allowButton),
		ui.LeftClick(allowButton),
	)(ctx); err != nil {
		s.Fatal("Failed to allow serial number permission: ", err)
	}

	if err := conn.Call(ctx, &resp,
		"tast.promisify(chrome.runtime.sendMessage)",
		"gogonhoemckpdpadfnjnpgbjpbjnodgc",
		"ping message",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}
	if diff := cmp.Diff(wantResp, resp); diff != "" {
		s.Error("Unexpected response from Telemetry extension (-want +got): ", diff)
	}

	s.Fatal("crash")
}

type swResponse struct {
	OemData  string   `json:"oemData"`
	VpdInfo  vpdInfo  `json:"vpdInfo"`
	Routines []string `json:"routines"`

	Error string `json:"error"`
}

// TODO(lamzin): add skuNumber
type vpdInfo struct {
	ActivateDate string `json:"activateDate"`
	ModelName    string `json:"modelName"`
	SerialNumber string `json:"serialNumber"`
}

func expectedSwResponse(ctx context.Context) (swResponse, error) {
	oemDataBytes, err := testexec.CommandContext(ctx, "/usr/share/cros/oemdata.sh").Output()
	if err != nil {
		return swResponse{}, errors.Wrap(err, "failed to get OEM data")
	}
	if len(oemDataBytes) == 0 {
		return swResponse{}, errors.New("OEM data is empty")
	}

	activateDateBytes, err := ioutil.ReadFile("/sys/firmware/vpd/rw/ActivateDate")
	if err != nil {
		return swResponse{}, errors.Wrap(err, "failed to read ActivateDate VPD field")
	}
	if len(activateDateBytes) == 0 {
		return swResponse{}, errors.New("ActivateDate VPD is empty")
	}

	modelNameBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/model_name")
	if err != nil {
		return swResponse{}, errors.Wrap(err, "failed to read model_name VPD field")
	}
	if len(modelNameBytes) == 0 {
		return swResponse{}, errors.New("model_name VPD is empty")
	}

	serialNumberBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/serial_number")
	if err != nil {
		return swResponse{}, errors.Wrap(err, "failed to read serial_number VPD field")
	}
	if len(serialNumberBytes) == 0 {
		return swResponse{}, errors.New("serial_number VPD is empty")
	}

	return swResponse{
		OemData: string(oemDataBytes),
		VpdInfo: vpdInfo{
			ActivateDate: string(activateDateBytes),
			ModelName:    string(modelNameBytes),
			SerialNumber: string(serialNumberBytes),
		},
		Routines: []string{
			"battery_capacity",
			"battery_health",
			"cpu_cache",
			"cpu_stress",
			"battery_discharge",
			"battery_charge",
			"memory",
		},
	}, nil
}
