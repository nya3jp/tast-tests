// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CCADocumentPerf,
		Desc:     "Measures the performance of document scanning library used in CCA",
		Contacts: []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:     []string{"document_256x256_P420.yuv", "document_2448x3264.jpg"},
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
	logPath := filepath.Join(s.OutDir(), "perf.log")
	if report, err := gtest.New(
		exec,
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.ExtraArgs(
			"--log_path="+logPath,
			"--jpeg_image="+s.DataPath("document_2448x3264.jpg"),
			"--nv12_image="+s.DataPath("document_256x256_P420.yuv"),
		),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
	}

	s.Log("Measuring document scanner performance report")
	if err := parseReportAndRecordMetrics(ctx, logPath, s.OutDir()); err != nil {
		s.Fatal("Failed to parse test log: ", err)
	}
}

func parseReportAndRecordMetrics(ctx context.Context, logPath, outputDir string) error {
	pv := perf.NewValues()

	file, err := os.Open(logPath)
	if err != nil {
		return errors.Wrap(err, "couldn't open log file")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.Split(line, ":")
		if len(tokens) != 2 {
			return errors.Errorf("wrong number of tokens in line %q", line)
		}
		msec, err := strconv.ParseUint(strings.TrimSpace(tokens[1]), 10, 32)
		if err != nil {
			return errors.Wrapf(err, "failed to parse time from line %q", line)
		}

		name := strings.TrimSpace(tokens[0])
		if name != "DetectFromNV12Image" && name != "DetectFromJPEGImage" && name != "DoPostProcessing" {
			return errors.Errorf("Unrecognized metrics name: %v", name)
		}
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, float64(msec))
	}
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "failed to scan test log")
	}

	if err := pv.Save(outputDir); err != nil {
		return errors.Wrap(err, "failed to save perf data")
	}
	return nil
}
