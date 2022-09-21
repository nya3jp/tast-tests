// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playback provides common code for video.Playback* tests.
package playback

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/tracing"
	"chromiumos/tast/testing"
)

// DecoderType represents the different video decoder types.
type DecoderType int

const (
	// Hardware means hardware-accelerated video decoding.
	Hardware DecoderType = iota
	// Software - Any software-based video decoder (e.g. ffmpeg, libvpx).
	Software
	// LibGAV1 is a subtype of the Software above, using an alternative library
	// to play AV1 video for experimentation purposes.
	// TODO(crbug.com/1047051): remove this flag when the experiment is over, and
	// turn DecoderType into a boolean to represent hardware or software decoding.
	LibGAV1
)

const (
	// Time to sleep while collecting data.
	// The time to wait just after stating to play video so that CPU usage gets stable.
	stabilizationDuration = 5 * time.Second
	// The time to wait after CPU is stable so as to measure solid metric values.
	measurementDuration = 25 * time.Second

	// TraceConfigFile is the perfetto config file to profile the scheduler events.
	TraceConfigFile = "perfetto_tbm_traced_probes.pbtxt"
	// GPUThreadSchedSQLFile is the sql script to count the number of context
	// switches and its waiting duration from the perfetto output.
	GPUThreadSchedSQLFile = "gpu_thread_sched.sql"
	// CPUIdleWakeupsSQLFile is the sql script to count the number of CPU idle
	// wake ups.
	CPUIdleWakeupsSQLFile = "cpu_idle_wakeups.sql"

	// Video Element in the page to play a video.
	videoElement = "document.getElementsByTagName('video')[0]"
)

type contextSwitchStat struct {
	count       uint64
	avgDuration time.Duration
}

// RunTest measures a number of performance metrics while playing a video with
// or without hardware acceleration as per decoderType.
func RunTest(ctx context.Context, s *testing.State, cs ash.ConnSource, cr *chrome.Chrome, videoName string, decoderType DecoderType, gridWidth, gridHeight int, perfTracing, measureRoughness bool) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer crastestclient.Unmute(ctx)

	s.Log("Starting playback")
	if err = measurePerformance(ctx, s, cs, cr, s.DataFileSystem(), videoName, decoderType, gridWidth, gridHeight, perfTracing, measureRoughness, s.OutDir()); err != nil {
		s.Fatal("Playback test failed: ", err)
	}
}

// measurePerformance collects video playback performance playing a video with
// either SW or HW decoder.
func measurePerformance(ctx context.Context, s *testing.State, cs ash.ConnSource, cr *chrome.Chrome, fileSystem http.FileSystem, videoName string,
	decoderType DecoderType, gridWidth, gridHeight int, perfTracing, measureRoughness bool, outDir string) error {
	// Wait until CPU is idle enough. CPU usage can be high immediately after login for various reasons (e.g. animated images on the lock screen).
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return err
	}

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	url := server.URL + "/video.html"
	conn, err := cs.NewConn(ctx, url)
	if err != nil {
		return errors.Wrap(err, "failed to open video page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	observer, err := conn.GetMediaPropertiesChangedObserver(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve DevTools Media messages")
	}

	// The page is already rendered with 1 video element by default.
	defaultGridSize := 1
	if gridWidth*gridHeight > defaultGridSize {
		if err := conn.Call(ctx, nil, "setGridSize", gridWidth, gridHeight); err != nil {
			return errors.Wrap(err, "failed to adjust the grid size")
		}
	}

	// Wait until video element(s) are loaded.
	exprn := fmt.Sprintf("document.getElementsByTagName('video').length == %d", int(math.Max(1.0, float64(gridWidth*gridHeight))))
	if err := conn.WaitForExpr(ctx, exprn); err != nil {
		return errors.Wrap(err, "failed to wait for video element loading")
	}

	// TODO(b/183044442): before playing and measuring, we should probably ensure
	// that the UI is in a known state.
	if err := conn.Call(ctx, nil, "playRepeatedly", videoName); err != nil {
		return errors.Wrap(err, "failed to start video")
	}

	// Wait until videoElement has advanced so that chrome:media-internals has
	// time to fill in their fields.
	if err := conn.WaitForExpr(ctx, videoElement+".currentTime > 1"); err != nil {
		return errors.Wrap(err, "failed waiting for video to advance playback")
	}

	isPlatform, decoderName, err := devtools.GetVideoDecoder(ctx, observer, url)
	if err != nil {
		return errors.Wrap(err, "failed to parse Media DevTools")
	}
	if decoderType == Hardware && !isPlatform {
		return errors.New("hardware decoding accelerator was expected but wasn't used")
	}
	if decoderType == Software && isPlatform {
		return errors.New("software decoding was expected but wasn't used")
	}
	testing.ContextLog(ctx, "decoderName: ", decoderName)
	if decoderType == LibGAV1 && decoderName != "Gav1VideoDecoder" {
		return errors.Errorf("Expect Gav1VideoDecoder, but used Decoder is %s", decoderName)
	}

	p := perf.NewValues()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}
	const decodeHistogram = "Media.MojoVideoDecoder.Decode"
	initDecodeHistogram, err := metrics.GetHistogram(ctx, tconn, decodeHistogram)
	if err != nil {
		return errors.Wrap(err, "failed to get initial histogram")
	}
	const platformdecodeHistogram = "Media.PlatformVideoDecoding.Decode"
	initPlatformdecodeHistogram, err := metrics.GetHistogram(ctx, tconn, platformdecodeHistogram)
	if err != nil {
		return errors.Wrap(err, "failed to get initial histogram")
	}

	var roughness float64
	var idleWakeups uint64
	var gpuCSStat, gpuDecCSStat contextSwitchStat
	var gpuErr, cStateErr, cpuErr, fdErr, dramErr, batErr, roughnessErr, traceErr error
	var wg sync.WaitGroup
	wg.Add(6)
	go func() {
		defer wg.Done()
		gpuErr = graphics.MeasureGPUCounters(ctx, measurementDuration, p)
	}()
	go func() {
		defer wg.Done()
		cStateErr = graphics.MeasurePackageCStateCounters(ctx, measurementDuration, p)
	}()
	go func() {
		defer wg.Done()
		cpuErr = graphics.MeasureCPUUsageAndPower(ctx, stabilizationDuration, measurementDuration, p)
	}()
	go func() {
		defer wg.Done()
		fdErr = graphics.MeasureFdCount(ctx, measurementDuration, p)
	}()
	go func() {
		defer wg.Done()
		dramErr = graphics.MeasureDRAMBandwidth(ctx, measurementDuration, p)
	}()
	go func() {
		defer wg.Done()
		batErr = graphics.MeasureSystemPowerConsumption(ctx, tconn, measurementDuration, p)
	}()
	if measureRoughness {
		wg.Add(1)

		go func() {
			defer wg.Done()
			// If the video sequence is not long enough, roughness won't be provided by
			// Media Devtools and this call will timeout.
			roughness, roughnessErr = devtools.GetVideoPlaybackRoughness(ctx, observer, url)
		}()
	}
	if perfTracing {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gpuCSStat, gpuDecCSStat, idleWakeups, traceErr = measureTraceEvents(ctx, s)
		}()
	}

	wg.Wait()
	if gpuErr != nil {
		return errors.Wrap(gpuErr, "failed to measure GPU counters")
	}
	if cStateErr != nil {
		return errors.Wrap(cStateErr, "failed to measure Package C-State residency")
	}
	if cpuErr != nil {
		return errors.Wrap(cpuErr, "failed to measure CPU/Package power")
	}
	if fdErr != nil {
		return errors.Wrap(fdErr, "failed to measure open FD count")
	}
	if dramErr != nil {
		return errors.Wrap(dramErr, "failed to measure DRAM bandwidth consumption")
	}
	if batErr != nil {
		return errors.Wrap(batErr, "failed to measure system power consumption")
	}
	if roughnessErr != nil {
		return errors.Wrap(roughnessErr, "failed to measure playback roughness")
	}
	if traceErr != nil {
		return errors.Wrap(traceErr, "failed to measure CPU sched events")
	}

	if err := graphics.UpdatePerfMetricFromHistogram(ctx, tconn, decodeHistogram, initDecodeHistogram, p, "video_decode_delay"); err != nil {
		return errors.Wrap(err, "failed to calculate Decode perf metric")
	}
	if err := graphics.UpdatePerfMetricFromHistogram(ctx, tconn, platformdecodeHistogram, initPlatformdecodeHistogram, p, "platform_video_decode_delay"); err != nil {
		return errors.Wrap(err, "failed to calculate Platform Decode perf metric")
	}

	if err := sampleDroppedFrames(ctx, conn, p); err != nil {
		return errors.Wrap(err, "failed to get dropped frames and percentage")
	}

	if measureRoughness {
		p.Set(perf.Metric{
			Name:      "roughness",
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, float64(roughness))
	}

	if perfTracing {
		p.Set(perf.Metric{
			Name:      "context_switches_in_gpu_process_cnt",
			Unit:      "count",
			Direction: perf.SmallerIsBetter,
		}, float64(gpuCSStat.count))
		p.Set(perf.Metric{
			Name:      "context_switches_in_gpu_process_avg_duration",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, float64(gpuCSStat.avgDuration.Milliseconds()))

		p.Set(perf.Metric{
			Name:      "context_switches_in_gpu_process_per_decoder_thread_cnt",
			Unit:      "count",
			Direction: perf.SmallerIsBetter,
		}, float64(gpuDecCSStat.count))
		p.Set(perf.Metric{
			Name:      "context_switches_in_gpu_process_per_decoder_thread_avg_duration",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, float64(gpuDecCSStat.avgDuration.Milliseconds()))
		p.Set(perf.Metric{
			Name:      "cpu_idle_wakeups",
			Unit:      "count",
			Direction: perf.SmallerIsBetter,
		}, float64(idleWakeups))
	}
	if err := conn.Eval(ctx, videoElement+".pause()", nil); err != nil {
		return errors.Wrap(err, "failed to stop video")
	}

	p.Save(outDir)
	return nil
}

// sampleDroppedFrames obtains the number of decoded and dropped frames.
func sampleDroppedFrames(ctx context.Context, conn *chrome.Conn, p *perf.Values) error {
	var decodedFrameCount, droppedFrameCount int64
	if err := conn.Eval(ctx, videoElement+".getVideoPlaybackQuality().totalVideoFrames", &decodedFrameCount); err != nil {
		return errors.Wrap(err, "failed to get number of decoded frames")
	}
	if err := conn.Eval(ctx, videoElement+".getVideoPlaybackQuality().droppedVideoFrames", &droppedFrameCount); err != nil {
		return errors.Wrap(err, "failed to get number of dropped frames")
	}

	var droppedFramePercent float64
	if decodedFrameCount != 0 {
		droppedFramePercent = 100.0 * float64(droppedFrameCount) / float64(decodedFrameCount)
	} else {
		testing.ContextLog(ctx, "No decoded frames; setting dropped percent to 100")
		droppedFramePercent = 100.0
	}

	p.Set(perf.Metric{
		Name:      "dropped_frames",
		Unit:      "frames",
		Direction: perf.SmallerIsBetter,
	}, float64(droppedFrameCount))
	p.Set(perf.Metric{
		Name:      "dropped_frames_percent",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, droppedFramePercent)

	testing.ContextLogf(ctx, "Dropped frames: %d (%f%%)", droppedFrameCount, droppedFramePercent)

	return nil
}

// computeContextSwitches acquires the number of thread context switches in GPU process
// and its average waiting duration from the tracing session.
func computeContextSwitches(ctx context.Context, s *testing.State, sess *tracing.Session) (gpu, gpuDec contextSwitchStat, err error) {
	results, err := sess.RunQuery(ctx, s.DataPath(tracing.TraceProcessor()), s.DataPath(GPUThreadSchedSQLFile))
	if err != nil {
		return gpu, gpuDec, errors.Wrap(err, "failed in querying")
	}

	var switches, runnableCnt, sumRunnableDur uint64 = 0, 0, 0
	var decThreadsSwitches, decThreadsRunnableCnt, decThreadsSumRunnableDur uint64 = 0, 0, 0
	for _, res := range results[1:] { // Skip, the first line, "ts","dur","state","tid","name".
		const (
			tsIdx = iota
			durIdx
			stateIdx
			tidIdx
			nameIdx
		)

		thName := res[nameIdx]
		isDecoderThread := strings.Contains(thName, "VDecThread")
		switch res[stateIdx] {
		case "Running":
			switches++
			if isDecoderThread {
				decThreadsSwitches++
			}
		case "R": // Runnable
			dur, err := strconv.Atoi(res[durIdx])
			if err != nil {
				return gpu, gpuDec, errors.Wrapf(err, "failed to convert to integer, %s", res[durIdx])
			}
			if dur == -1 {
				// dur is -1 if tracing terminates while a thread is in the Runnable state.
				continue
			}
			runnableCnt++
			sumRunnableDur += uint64(dur)
			if isDecoderThread {
				decThreadsRunnableCnt++
				decThreadsSumRunnableDur += uint64(dur)
			}
		}
	}

	gpu.count = switches
	gpu.avgDuration = time.Duration(sumRunnableDur/runnableCnt) * time.Microsecond
	gpuDec.count = decThreadsSwitches
	gpuDec.avgDuration = time.Duration(decThreadsSumRunnableDur/decThreadsRunnableCnt) * time.Microsecond
	return gpu, gpuDec, nil
}

func computeIdleWakeups(ctx context.Context, s *testing.State, sess *tracing.Session) (idleWakeups uint64, err error) {
	results, err := sess.RunQuery(ctx, s.DataPath(tracing.TraceProcessor()), s.DataPath(CPUIdleWakeupsSQLFile))
	if err != nil {
		return idleWakeups, errors.Wrap(err, "failed in querying")
	}

	for i, res := range results[1:] { // Skip, the first line, "utid", "name", "name", "count(*)"
		const (
			utidIdx = iota
			thNameIdx
			prNameIdx
			cntIdx
		)
		thName := res[thNameIdx]
		prName := res[prNameIdx]

		cnt, err := strconv.ParseUint(res[cntIdx], 10, 64)
		if err != nil {
			return idleWakeups, errors.Wrapf(err, "failed to convert to integer, %s", res[cntIdx])
		}

		// Output the top 10 threads causing the cpu idle wakeups.
		if i < 10 {
			testing.ContextLogf(ctx, "%d: thread=%s, process=%s, wakeups=%d", i, thName, prName, cnt)
		}

		// Don't count wakeups by traced and traced_probes.
		if !strings.Contains(prName, "traced") {
			idleWakeups += cnt
		}

	}
	testing.ContextLogf(ctx, "idle wakeups: %d", idleWakeups)
	return idleWakeups, nil
}

// measureTraceEvents gets the following metrics using tracing:
// * the number of thread context switches in GPU process
// * the average waiting duration of the switches
// * the number of cpu idle wakeups on the entire system
func measureTraceEvents(ctx context.Context, s *testing.State) (gpu, gpuDec contextSwitchStat, idleWakeups uint64, err error) {
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		return gpu, gpuDec, idleWakeups, err
	}

	testing.ContextLog(ctx, "Tracing scheduler events")
	// Record system events for |measurementDuration|.
	sess, err := tracing.StartSession(ctx, s.DataPath(TraceConfigFile))
	if err != nil {
		return gpu, gpuDec, idleWakeups, errors.Wrap(err, "failed to start tracing")
	}
	// Stop tracing even if context deadline exceeds during sleep.
	stopped := false
	defer func() {
		if !stopped {
			sess.Stop()
		}
	}()

	if err := testing.Sleep(ctx, measurementDuration); err != nil {
		return gpu, gpuDec, idleWakeups, errors.Wrap(err, "failed to sleep to wait for the tracing session")
	}
	stopped = true
	if err := sess.Stop(); err != nil {
		return gpu, gpuDec, idleWakeups, errors.Wrap(err, "failed to stop tracing")
	}
	defer sess.RemoveTraceResultFile()
	testing.ContextLog(ctx, "Completed tracing events")

	gpu, gpuDec, err = computeContextSwitches(ctx, s, sess)
	if err != nil {
		return gpu, gpuDec, idleWakeups, errors.Wrap(err, "failed to get context switches")
	}
	idleWakeups, err = computeIdleWakeups(ctx, s, sess)
	if err != nil {
		return gpu, gpuDec, idleWakeups, errors.Wrap(err, "failed to get cpu idle wakeups")

	}
	return gpu, gpuDec, idleWakeups, nil
}
