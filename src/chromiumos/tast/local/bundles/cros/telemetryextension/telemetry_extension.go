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

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	manifestJSON = "manifest.json"
	optionsHTML  = "options.html"
	swJS         = "sw.js"

	testExtensionID = "gogonhoemckpdpadfnjnpgbjpbjnodgc"
)

var dataFiles = []string{manifestJSON, optionsHTML, swJS}

func init() {
	testing.AddTest(&testing.Test{
		Func:         TelemetryExtension,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Tests TelemetryExtension core functionalities such as APIs, permissions, communication with PWA",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         dataFiles,
		Params: []testing.Param{
			{
				Name:              "target_models",
				ExtraHardwareDeps: dep.TargetModels(),
			},
			{
				Name:              "low_priority_target_models",
				ExtraHardwareDeps: dep.LowPriorityTargetModels(),
			},
		},
		Timeout: 4*time.Hour,
	})
}

// TelemetryExtension tests that Telemetry Extension has access to APIs and can talk with PWA.
func TelemetryExtension(ctx context.Context, s *testing.State) {
	// Load want response first to be sure that DUT satisfies all requirements to run Telemetry Extension.
	// wantResp, err := expectedSwResponse(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to get expected response: ", err)
	// }

	dir, err := ioutil.TempDir("", "telemetry_extension")
	if err != nil {
		s.Fatal("Failed to create temporary directory for TelemetryExtension: ", err)
	}
	// defer os.RemoveAll(dir)

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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connections: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Connecting to google.com")
	conn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create connection to google.com: ", err)
	}
	defer conn.Close()

	s.Log("Adding tast library")
	if err := chrome.AddTastLibrary(ctx, conn); err != nil {
		s.Fatal("Failed to add Tast library to PWA: ", err)
	}

	powerMetrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(1*time.Minute))
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if err := powerMetrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	s.Log("Starting measurement")
	if err := powerMetrics.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	defer func(ctx context.Context){
		p, err := powerMetrics.StopRecording(ctx)
		if err != nil {
			s.Fatal("Error while recording power metrics: ", err)
		}

		if err := p.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}(ctx)

	// Doing smth in the background
	//
	// ~ 360 cycle x 10 sec = 1 hour 
	for i := 0; i < 360; i++ {
		now := time.Now()

		var resp swResponse
		if err := conn.Call(ctx, &resp,
			"tast.promisify(chrome.runtime.sendMessage)",
			testExtensionID,
			"ping message",
		); err != nil {
			s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
		}

		s.Log("i =", i)
		s.Log(resp.ApiStats)
		s.Log("JS call time: ", time.Now().Sub(now))

		time.Sleep(8700*time.Millisecond)
	}
}

type swResponse struct {
	ApiStats string   `json:"apiStats"`
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
			// SkuNumber:    string(skuNumberBytes),
		},
		Routines: []string{
			"battery_capacity",
			"battery_health",
			"cpu_cache",
			"cpu_stress",
			"cpu_floating_point_accuracy",
			"cpu_prime_search",
			"battery_discharge",
			"battery_charge",
			"memory",
		},
	}, nil
}
