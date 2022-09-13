// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"strconv"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/getusermedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/tracing"
	"chromiumos/tast/testing"
)

type metricsPath struct {
	trace     string
	processor string
	query     string
	outputDir string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GetUserMediaPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Captures performance data about getUserMedia video capture",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{caps.BuiltinOrVividCamera, "chrome"},
		Data: append(
			getusermedia.DataFiles(),
			"getusermedia.html",
			"perfetto/camera_config.pbtxt",
			"perfetto/camera_query.sql",
			tracing.TraceProcessorAmd64,
			tracing.TraceProcessorArm,
			tracing.TraceProcessorArm64),
		Params: []testing.Param{
			{
				Pre: pre.ChromeCameraPerf(),
				Val: browser.TypeAsh,
			},
			{
				Name:              "lacros",
				Fixture:           "chromeCameraPerfLacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Timeout:           7 * time.Minute, // A lenient limit for launching Lacros Chrome.
				Val:               browser.TypeLacros,
			},
		},
	})
}

func collectMetrics(ctx context.Context, pv *perf.Values, sess *tracing.Session, paths metricsPath) error {
	// Transfer trace file to output directory.
	defer os.Rename(sess.TraceResultFile.Name(), paths.trace)

	if err := sess.Stop(); err != nil {
		return errors.Wrap(err, "failed to stop session")
	}

	// Collect important metrics and upload to CrosBolt.
	metrics, err := sess.RunQuery(ctx, paths.processor, paths.query)
	if err != nil {
		return errors.Wrap(err, "failed to process the trace data")
	}

	names := metrics[0]
	if names[0] != "open_device" || names[1] != "configure_streams" {
		return errors.Wrap(err, "unexpected query column names")
	}

	values := metrics[1]
	if len(names) != len(values) {
		return errors.Wrap(err, "mismatched amount of columns")
	}

	for i := 0; i < len(names); i++ {
		value, err := strconv.ParseFloat(values[i], 64)
		if err != nil {
			return errors.Wrapf(err, "value is not float64: %v", values[i])
		}

		pv.Set(perf.Metric{
			Name:      names[i],
			Unit:      "nanosecond",
			Direction: perf.SmallerIsBetter,
		}, value)
	}
	return pv.Save(paths.outputDir)
}

// GetUserMediaPerf is the full version of GetUserMedia. It renders the camera's
// media stream in VGA and 720p for 20 seconds. If there is no error while
// exercising the camera, it uploads statistics of black/frozen frames. This
// test will fail when an error occurs or too many frames are broken.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func GetUserMediaPerf(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	traceConfigPath := s.DataPath("perfetto/camera_config.pbtxt")
	sess, err := tracing.StartSession(ctx, traceConfigPath)
	if err != nil {
		s.Fatal("Failed to start tracing: ", err)
	}

	p := perf.NewValues()
	paths := metricsPath{
		trace:     s.OutDir() + "/trace.pb",
		processor: s.DataPath(tracing.TraceProcessor()),
		query:     s.DataPath("perfetto/camera_query.sql"),
		outputDir: s.OutDir()}
	defer func(cleanupCtx context.Context) {
		if s.HasError() {
			return
		}
		if err := collectMetrics(cleanupCtx, p, sess, paths); err != nil {
			s.Fatal("Failed to collect metrics: ", err)
		}
	}(cleanupCtx)

	s.Log("Collecting Perfetto trace File at: ", sess.TraceResultFile.Name())

	var ci getusermedia.ChromeInterface
	runLacros := s.Param().(browser.Type) == browser.TypeLacros
	if runLacros {
		tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect to test API: ", err)
		}

		ci, err = lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch lacros-chrome: ", err)
		}
		defer ci.Close(ctx)
	} else {
		ci = s.PreValue().(*chrome.Chrome)
	}

	// Run tests for 20 seconds per resolution.
	results := getusermedia.RunGetUserMedia(ctx, s, ci, 20*time.Second, getusermedia.NoVerboseLogging)

	if !s.HasError() {
		// Set and upload frame statistics below.
		results.SetPerf(p)
	}
}
