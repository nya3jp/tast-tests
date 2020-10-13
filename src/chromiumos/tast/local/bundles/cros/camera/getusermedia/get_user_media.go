// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package getusermedia provides code for webrtc.* tests related to getUserMedia(), see:
// https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getUserMedia.
package getusermedia

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

// RunDecodeAccelUsedJPEG tests that the HW JPEG decoder is used in a GetUserMedia().
// The test fails if bucketValue on histogramName does not count up.
func RunDecodeAccelUsedJPEG(ctx context.Context, s *testing.State, getUserMediaFilename, streamName, histogramName string, bucketValue int64) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := openPageAndCheckBucket(ctx, s.DataFileSystem(), getUserMediaFilename, s.DataPath(streamName), histogramName, bucketValue); err != nil {
		s.Fatal("Failed: ", err)
	}
}

// openPageAndCheckBucket opens getUserMediaFilename, and uses GetUserMedia() to
// stream streamFile. Then it verifies that bucketValue on histogramName counts
// up in the end of the test.
func openPageAndCheckBucket(ctx context.Context, fileSystem http.FileSystem, getUserMediaFilename, streamFile, histogramName string, bucketValue int64) error {
	chromeArgs := webrtc.ChromeArgsWithFileCameraInput(streamFile, true)
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	initHistogram, err := metrics.GetHistogram(ctx, tconn, histogramName)
	if err != nil {
		return errors.Wrap(err, "failed to get initial histogram")
	}
	testing.ContextLogf(ctx, "Initial %s histogram: %v", histogramName, initHistogram.Buckets)

	conn, err := cr.NewConn(ctx, server.URL+"/"+getUserMediaFilename)
	if err != nil {
		return errors.Wrapf(err, "failed to open page %s", getUserMediaFilename)
	}
	defer conn.Close()
	// Close the tab to stop loopback after test.
	defer conn.CloseTarget(ctx)

	if err := conn.Eval(ctx, `(async() => {
		  const constraints = {audio: false, video: true};
		  const stream = await navigator.mediaDevices.getUserMedia(constraints);
		  document.getElementById('localVideo').srcObject = stream;
		})()`, nil); err != nil {
		return errors.Wrap(err, "getUserMedia() establishment failed")
	}

	histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, tconn, histogramName, initHistogram, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed getting histogram diff")
	}

	if len(histogramDiff.Buckets) > 1 {
		return errors.Wrapf(err, "unexpected histogram update: %v", histogramDiff)
	}

	bucket := histogramDiff.Buckets[0]
	// Expected histogram is [bucketValue, bucketValue+1, 1].
	if bucket.Min != bucketValue || bucket.Max != bucketValue+1 || bucket.Count != 1 {
		return errors.Wrapf(err, "unexpected histogram update: %v", bucket)
	}

	return nil
}

// cameraResults is a type for decoding JSON objects obtained from /data/getusermedia.html.
type cameraResults []struct {
	Width      int        `json:"width"`
	Height     int        `json:"height"`
	FrameStats FrameStats `json:"frameStats"`
	Errors     []string   `json:"errors"`
}

// setPerf stores performance data of cameraResults into p.
func (r *cameraResults) SetPerf(p *perf.Values) {
	for _, result := range *r {
		perfSuffix := fmt.Sprintf("%dx%d", result.Width, result.Height)
		result.FrameStats.SetPerf(p, perfSuffix)
	}
}

// VerboseLoggingMode describes whether video driver's verbose debug log is enabled.
type VerboseLoggingMode int

const (
	// VerboseLogging enables verbose logging.
	VerboseLogging VerboseLoggingMode = iota
	// NoVerboseLogging disables verbose logging.
	NoVerboseLogging
)

// RunGetUserMedia run a test in /data/getusermedia.html.
// duration specifies how long video capturing will run for each resolution.
// If verbose is true, video drivers' verbose messages will be enabled.
// verbose must be false for performance tests.
func RunGetUserMedia(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	duration time.Duration, verbose VerboseLoggingMode) cameraResults {
	if verbose == VerboseLogging {
		vl, err := logging.NewVideoLogger()
		if err != nil {
			s.Fatal("Failed to set values for verbose logging")
		}
		defer vl.Close()
	}

	var results cameraResults
	RunTest(ctx, s, cr, "getusermedia.html", fmt.Sprintf("testNextResolution(%d)", duration/time.Second), &results)

	s.Logf("Results: %+v", results)

	for _, result := range results {
		if len(result.Errors) != 0 {
			for _, msg := range result.Errors {
				s.Errorf("%dx%d: %s", result.Width, result.Height, msg)
			}
		}

		if err := result.FrameStats.CheckTotalFrames(); err != nil {
			s.Errorf("%dx%d was not healthy: %v", result.Width, result.Height, err)
		}
		// Only check the percentage of broken and black frames if we are
		// running under QEMU, see crbug.com/898745.
		if vm.IsRunningOnVM() {
			if err := result.FrameStats.CheckBrokenFrames(); err != nil {
				s.Errorf("%dx%d was not healthy: %v", result.Width, result.Height, err)
			}
		}
	}

	return results
}

// getAllPIDs returns all PIDs with name.
func getAllPIDs(name string) ([]int, error) {
	var pids []int
	procs, err := process.Processes()
	if err != nil {
		return pids, err
	}
	for _, proc := range procs {
		if cl, err := proc.Cmdline(); err != nil || !strings.Contains(cl, name) {
			continue
		}
		pids = append(pids, int(proc.Pid))
	}
	if len(pids) == 0 {
		return pids, errors.Errorf("unable to find process with name %v", name)
	}
	return pids, nil
}
func getAllCameraRelatedProcesses() ([]string, [][]int, error) {
	var pnames []string
	var pids [][]int
	for _, pname := range []string{"cros_camera_service", "cros_camera_algo"} {
		curPids, err := getAllPIDs(pname)
		if err != nil {
			return pnames, pids, err
		}
		pnames = append(pnames, pname)
		pids = append(pids, curPids)
	}
	crBrowserPID, err := chrome.GetRootPID()
	if err != nil {
		return pnames, pids, errors.Wrap(err, "failed getting chrome browser process id")
	}
	pnames = append(pnames, "chrome_browser")
	pids = append(pids, []int{crBrowserPID})
	return pnames, pids, nil

}

func runQuipper(ctx context.Context, pids []int, duration int, pname, outDir string) error {
	testing.ContextLogf(ctx, "Executing quipper to %s (pids: %v)", pname, pids)
	output, err := testexec.CommandContext(
		ctx,
		"quipper", strconv.Itoa(duration), "perf", "record", "-a", "-g",
		"-F", "499",
		"-p", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(pids)), ","), "[]")).Output()
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "size=%d", len(output))
	outFile := filepath.Join(outDir, fmt.Sprintf("%s.proto", pname))
	if err := ioutil.WriteFile(outFile, output, 0644); err != nil {
		return errors.Wrap(err, "Failed writing proto to the file")
	}
	return nil
}

func RunGetUserMediaPerf2(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	const quipperDuration = 5

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	conn, err := cr.NewConn(ctx, server.URL+"/camera_preview.html")
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	shortCtx, rcancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer rcancel()

	if err := conn.WaitForExpr(shortCtx, "scriptReady"); err != nil {
		s.Fatal("Timed out waiting for scripts ready: ", err)
	}
	if err := conn.WaitForExpr(shortCtx, "cameraReady()"); err != nil {
		s.Fatal("Timed out waiting for camera ready: ", err)
	}

	var cameraWidth, cameraHeight int
	if err := conn.Eval(shortCtx, "video.videoWidth", &cameraWidth); err != nil {
		s.Fatal("Failed getting the camera width: ", err)
	}
	if err := conn.Eval(shortCtx, "video.videoHeight", &cameraHeight); err != nil {
		s.Fatal("Failed getting the camera height: ", err)
	}

	testing.ContextLogf(shortCtx, "camera resolution: %dx%d", cameraWidth, cameraHeight)

	pnames, pids, err := getAllCameraRelatedProcesses()
	if err != nil {
		s.Fatal("Failed to get all camera related processes: ", err)
	}
	for i := 0; i < len(pnames); i++ {
		pname := pnames[i]
		curPids := pids[i]
		if err := runQuipper(shortCtx, curPids, quipperDuration, pname, s.OutDir()); err != nil {
			s.Fatal("Failed to execute quipper: ", err)
		}
	}
}
