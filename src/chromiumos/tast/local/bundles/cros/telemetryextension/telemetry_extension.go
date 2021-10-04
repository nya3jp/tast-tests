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
		Desc: "Tests TelemetryExtension core functionalities such as APIs, permissions, communication with PWA",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         dataFiles,
	})
}

// TelemetryExtension tests that Telemetry Extension has access to APIs and can talk with PWA.
func TelemetryExtension(ctx context.Context, s *testing.State) {
	dir, err := ioutil.TempDir("", "telemetry_extension")
	if err != nil {
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

	cr, err := chrome.New(ctx, chrome.UnpackedExtension(dir))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create connection to google.com: ", err)
	}
	defer conn.Close()

	const js = `
		new Promise((resolve, reject) => {
			chrome.runtime.sendMessage('gogonhoemckpdpadfnjnpgbjpbjnodgc', {'msg': 'ping'},
				function(response) {
					resolve(response);
				}
			);
		})
	`

	var resp swResponse
	if err := conn.Eval(ctx, js, &resp); err != nil {
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

	optionsConn, err := cr.NewConn(ctx, "chrome://extensions/?id=gogonhoemckpdpadfnjnpgbjpbjnodgc")
	if err != nil {
		s.Fatal("Failed to create connection to Chrome extensions page: ", err)
	}
	defer optionsConn.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connections: ", err)
	}

	// Request and accept "os.telemetry.serial_number" permission in order to
	// get access to serial number and OEM data (e.g. battery serial number).
	ui := uiauto.New(tconn)
	optionsButton := nodewith.Name("Extension options").Role(role.Link)
	requestButton := nodewith.Name("Request serial number permission").Role(role.Button)
	allowButton := nodewith.Name("Allow").Role(role.Button)
	if err := uiauto.Combine("allow serial number permission",
		ui.WithTimeout(5*time.Second).WaitUntilExists(optionsButton),
		ui.LeftClick(optionsButton),
		ui.WithTimeout(5*time.Second).WaitUntilExists(requestButton),
		ui.LeftClick(requestButton),
		ui.WithTimeout(5*time.Second).WaitUntilExists(allowButton),
		ui.LeftClick(allowButton),
	)(ctx); err != nil {
		s.Fatal("Failed to allow serial number permission: ", err)
	}

	if err := conn.Eval(ctx, js, &resp); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}
	if diff := cmp.Diff(wantResp, resp); diff != "" {
		s.Error("Unexpected response from Telemetry extension (-want +got): ", diff)
	}
}

type swResponse struct {
	OemData  string   `json:"oemData"`
	VpdInfo  vpdInfo  `json:"vpdInfo"`
	Routines []string `json:"routines"`

	Error string `json:"error"`
}

type vpdInfo struct {
	ActivateDate string `json:"activateDate"`
	ModelName    string `json:"modelName"`
	SerialNumber string `json:"serialNumber"`
	SkuNumber    string `json:"skuNumber"`
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

	skuNumberBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/sku_number")
	if err != nil {
		return swResponse{}, errors.Wrap(err, "failed to read sku_number VPD field")
	}
	if len(skuNumberBytes) == 0 {
		return swResponse{}, errors.New("sku_number VPD is empty")
	}

	return swResponse{
		OemData: string(oemDataBytes),
		VpdInfo: vpdInfo{
			ActivateDate: string(activateDateBytes),
			ModelName:    string(modelNameBytes),
			SerialNumber: string(serialNumberBytes),
			SkuNumber:    string(skuNumberBytes),
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
