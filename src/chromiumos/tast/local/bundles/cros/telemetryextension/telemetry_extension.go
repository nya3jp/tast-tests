// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package telemetryextension contains tests for Telemetry Extension.
package telemetryextension

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	manifestJSON = "manifest.json"
	swJS         = "sw.js"
)

var dataFiles = []string{manifestJSON, swJS}

func init() {
	testing.AddTest(&testing.Test{
		Func: TelemetryExtension,
		Desc: "Tests TelemetryExtension code functionality",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
		},
		SoftwareDeps: []string{"chrome"},
		Data:         dataFiles,
	})
}

// TelemetryExtension tests that Telemetry Extension has access to APIs and can talk with PWA.
func TelemetryExtension(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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
	defer cr.Close(cleanupCtx)

	conn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create connection to telemetry extension: ", err)
	}
	defer conn.Close()

	js := `
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

	wantResp, err := wantSwResponse(ctx)
	if err != nil {
		s.Fatal("Failed to get expected response: ", err)
	}

	if !reflect.DeepEqual(resp, wantResp) {
		s.Fatalf("Unexpected response from Telemetry extension: got %v; want %v", resp, wantResp)
	}
}

type swResponse struct {
	OemData  string   `json:"oemData"`
	VpdInfo  vpdInfo  `json:"vpdInfo"`
	Routines []string `json:"routines"`
}

// TODO(lamzin): add skuNumber
type vpdInfo struct {
	ActivateDate string `json:"activateDate"`
	ModelName    string `json:"modelName"`
	SerialNumber string `json:"serialNumber"`
}

func wantSwResponse(ctx context.Context) (swResponse, error) {
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

	modelNameBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/model_name")
	if err != nil {
		return swResponse{}, errors.Wrap(err, "failed to read model_name VPD field")
	}

	serialNumberBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/serial_number")
	if err != nil {
		return swResponse{}, errors.Wrap(err, "failed to read serial_number VPD field")
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
