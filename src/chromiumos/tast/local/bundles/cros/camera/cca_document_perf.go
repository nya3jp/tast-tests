// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCADocumentPerf,
		Desc:         "Measures the performance of document scanning library used in CCA",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         []string{"document_256x256_P420.yuv", "document_2448x3264.jpg"},
		SoftwareDeps: []string{"ondevice_document_scanner"},
		Timeout:      4 * time.Minute,
	})
}

// CCADocumentPerf runs the perf test which exercises document scanner library
// directly and send the performance metrics to CrosBolt.
func CCADocumentPerf(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(cleanupCtx, "ui")

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(cleanupCtx)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	s.Log("Measuring document scanner performance")
	const exec = "document_scanner_perf_test"
	outputPath := filepath.Join(s.OutDir(), "perf.json")
	if report, err := gtest.New(
		exec,
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.ExtraArgs(
			"--output_path="+outputPath,
			"--jpeg_image="+s.DataPath("document_2448x3264.jpg"),
			"--nv12_image="+s.DataPath("document_256x256_P420.yuv"),
		),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
	}

	s.Log("Parsing document scanner performance report")
	if err := parseReportAndRecordMetrics(ctx, outputPath, s.OutDir()); err != nil {
		s.Fatal("Failed to parse test log: ", err)
	}
}

func parseReportAndRecordMetrics(ctx context.Context, outputPath, outputDir string) error {
	pv := perf.NewValues()

	b, err := ioutil.ReadFile(outputPath)
	if err != nil {
		return errors.Wrap(err, "cannot read log file")
	}
	var metrics map[string]float64
	if err := json.Unmarshal(b, &metrics); err != nil {
		return errors.Wrap(err, "failed to unmarshal performance metrics")
	}

	for k, v := range metrics {
		if k != "DetectNV12Image" && k != "DetectJPEGImage" && k != "DoPostProcessing" {
			return errors.Errorf("Unrecognized metrics name: %v", k)
		}
		testing.ContextLogf(ctx, "Perf: %v => %v ms", k, v)
		pv.Set(perf.Metric{
			Name:      k,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, v)
	}

	if err := pv.Save(outputDir); err != nil {
		return errors.Wrap(err, "failed to save perf data")
	}
	return nil
}
