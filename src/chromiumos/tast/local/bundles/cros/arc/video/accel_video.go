// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package video provides common code to run ARC binary tests for video encoding.
package video

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/c2e2etest"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// cpuLog is the name of log file recording CPU usage.
const cpuLog = "cpu.log"

// powerLog is the name of lof file recording power consumption.
const powerLog = "power.log"

// binArgs is the arguments and the modes for executing video_encode_accelerator_unittest binary.
type binArgs struct {
	// testFilter specifies test pattern in googletest style for the unittest to run and will be passed with "--gtest_filter" (see go/gtest-running-subset).
	// If unspecified, the unittest runs all tests.
	testFilter string
	// extraArgs is the additional arguments to pass video_encode_accelerator_unittest, for example, "--native_input".
	extraArgs []string
	// measureUsage indicates whether to measure CPU usage and power consumption while running binary and save as perf metrics.
	measureUsage bool
	// measureDuration specifies how long to measure CPU usage and power consumption when measureUsage is set.
	measureDuration time.Duration
}

// testMode represents the test's running mode.
type testMode int

const (
	// functionalTest indicates a functional test.
	functionalTest testMode = iota
	// performanceTest indicates a performance test. CPU scaling should be adujst to performance.
	performanceTest
)

// runARCVideoEncoderTest runs arcvideoencoder_test in ARC.
// It pushes the binary files with different ABI and testing video data into ARC, and runs each binary for each binArgs.
// pv is optional value, passed when we run performance test and record measurement value.
// Note: pv must be provided when measureUsage is set at binArgs.
func runARCVideoEncoderTest(ctx context.Context, s *testing.State, a *arc.ARC, opts encoding.TestOptions, pv *perf.Values, bas ...binArgs) {
	// Install the test APK and grant permissions
	apkName, err := c2e2etest.ApkNameForArch(ctx, a)
	if err != nil {
		s.Fatal("Failed to get apk: ", err)
	}

	s.Log("Installing APK ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Granting storage permissions")
	if err := c2e2etest.GrantApkPermissions(ctx, a); err != nil {
		s.Fatal("Failed granting storage permission: ", err)
	}

	// Prepare video stream.
	params := opts.Params
	streamPath, err := encoding.PrepareYUV(ctx, s.DataPath(params.Name), opts.PixelFormat, params.Size)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(streamPath)

	// Push video stream file to ARC container.
	if err := a.Command(ctx, "mkdir", arcFilePath).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed creating test data dir: ", err)
	}
	defer a.Command(ctx, "rm", "-rf", arcFilePath).Run()

	if err := a.PushFile(ctx, streamPath, arcFilePath); err != nil {
		s.Fatal("Failed to push video stream to ARC: ", err)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if opts.Profile != videotype.H264Prof {
		s.Fatalf("Profile (%d) is not supported", opts.Profile)
	}
	encodeOutFile := strings.TrimSuffix(params.Name, ".vp9.webm") + ".h264"
	outPath := filepath.Join(arcFilePath, encodeOutFile)

	commonArgs := []string{
		encoding.CreateStreamDataArg(params, opts.Profile, opts.PixelFormat, arcFilePath+"/"+filepath.Base(streamPath), outPath),
	}
	for _, ba := range bas {
		if err := runARCBinaryWithArgs(ctx, s, a, commonArgs, ba, pv); err != nil {
			s.Errorf("Failed to run test with %v: %v", ba, err)
		}
	}
}

// runARCBinaryWithArgs runs arcvideoencoder_test binary with one binary argument.
// pv is optional value, passed when we run performance test and record measurement value.
// Note: pv must be provided when measureUsage is set at binArgs.
func runARCBinaryWithArgs(ctx context.Context, s *testing.State, a *arc.ARC, commonArgs []string, ba binArgs, pv *perf.Values) error {
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	nowStr := time.Now().Format("20060102-150405")
	outputLogFileName := fmt.Sprintf("output_%s.log", nowStr)
	outputXMLFileName := fmt.Sprintf("output_%s.xml", nowStr)

	s.Log("Starting APK main activity")
	act, err := arc.NewActivity(a, c2e2etest.Pkg, c2e2etest.ActivityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()
	args := append([]string(nil), commonArgs...)
	args = append(args, ba.extraArgs...)
	args = append(args, "--gtest_filter="+ba.testFilter)
	args = append(args, "--gtest_output=xml:"+arcFilePath+outputXMLFileName)
	if err := act.StartWithArgs(ctx, tconn, []string{"-W", "-n"}, []string{
		"--ez", "do-encode", "true",
		"--esa", "test-args", strings.Join(args, ","),
		"--es", "log-file", arcFilePath + outputLogFileName}); err != nil {
		s.Fatal("Failed starting APK main activity: ", err)
	}

	const schemaName = "c2e2etest"
	if ba.measureUsage {
		if pv == nil {
			return errors.New("pv should not be nil when measuring CPU usage and power consumption")
		}

		measurements, err := cpu.MeasureUsage(ctx, ba.measureDuration)
		if err != nil {
			return errors.Wrapf(err, "failed to run (measure CPU and power consumption): %v", err)
		}
		cpuUsage := measurements["cpu"]
		// TODO(b/143190876): Don't write value to disk, as this can increase test flakiness.
		cpuLogPath := filepath.Join(s.OutDir(), cpuLog)
		if err := ioutil.WriteFile(cpuLogPath, []byte(fmt.Sprintf("%f", cpuUsage)), 0644); err != nil {
			return errors.Wrap(err, "failed to write CPU usage to file")
		}

		if err := encoding.ReportCPUUsage(pv, schemaName, cpuLogPath); err != nil {
			return errors.Wrap(err, "failed to report CPU usage")
		}

		powerConsumption, ok := measurements["power"]
		if ok {
			// TODO(b/143190876): Don't write value to disk, as this can increase test flakiness.
			powerLogPath := filepath.Join(s.OutDir(), powerLog)
			if err := ioutil.WriteFile(powerLogPath, []byte(fmt.Sprintf("%f", powerConsumption)), 0644); err != nil {
				return errors.Wrap(err, "failed to write power consumption to file")
			}

			if err := encoding.ReportPowerConsumption(pv, schemaName, powerLogPath); err != nil {
				return errors.Wrap(err, "failed to report power consumption")
			}
		}
	} else {
		s.Log("Waiting for activity to finish")
		if err := act.WaitForFinished(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for activity: ", err)
		}

		// TODO Sleep to catch some extra logs.
		s.Log("Sleeping 5s to catch some extra logs...")
		time.Sleep(5 * time.Second)

		localOutputLogFile, localXMLLogFile, err := c2e2etest.PullLogs(ctx, a, arcFilePath, s.OutDir(), "", outputLogFileName, outputXMLFileName)

		if err != nil {
			s.Fatal("Failed to pull logs: ", err)
		}

		if err := c2e2etest.ValidateXMLLogs(localXMLLogFile); err != nil {
			s.Fatal("Failed to validate logs: ", err)
		}

		// Parse the performance result.
		if pv != nil {
			if err := encoding.ReportFPS(pv, schemaName, localOutputLogFile); err != nil {
				return errors.Wrap(err, "failed to report FPS value")
			}
			if err := encoding.ReportEncodeLatency(pv, schemaName, localOutputLogFile); err != nil {
				return errors.Wrap(err, "failed to report encode latency")
			}
		}
	}
	return nil
}

// RunARCVideoTest runs all non-perf tests of arcvideoencoder_test in ARC.
func RunARCVideoTest(ctx context.Context, s *testing.State, a *arc.ARC, opts encoding.TestOptions) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	runARCVideoEncoderTest(ctx, s, a, opts, nil, binArgs{testFilter: "C2VideoEncoderE2ETest.TestSimpleEncode"})
}

// RunARCPerfVideoTest runs all perf tests of arcvideoencoder_test in ARC.
func RunARCPerfVideoTest(ctx context.Context, s *testing.State, a *arc.ARC, opts encoding.TestOptions) {
	const (
		// duration of the interval during which CPU usage will be measured.
		measureDuration = 10 * time.Second
		// time reserved for cleanup.
		cleanupTime = 5 * time.Second
	)

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	// Leave a bit of time to clean up benchmark mode.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	pv := perf.NewValues()
	runARCVideoEncoderTest(ctx, s, a, opts, pv,
		// Measure FPS and latency.
		binArgs{
			testFilter: "C2VideoEncoderE2ETest.Perf*",
		},
		// Measure CPU usage.
		binArgs{
			testFilter:      "C2VideoEncoderE2ETest.TestSimpleEncode",
			extraArgs:       []string{"--run_at_fps", "--num_encoded_frames=10000"},
			measureUsage:    true,
			measureDuration: measureDuration,
		})
	pv.Save(s.OutDir())
}
