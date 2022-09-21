// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perfetto provides set of util functions used to run perfetto tool set.
package perfetto

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/metrics/github.com/google/perfetto/perfetto_proto"
	"github.com/gogo/protobuf/proto"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
)

const traceProcessorURL = "https://get.perfetto.dev/trace_processor"

var (
	tmpDir             = filepath.Join(os.TempDir(), "arc_perfetto")
	traceProcessorPath = filepath.Join(tmpDir, "trace_processor")
)

// Metrics defined in external/perfetto/protos/perfetto/metrics/metrics.proto
// of Android repo.
const (
	AndroidBatteryMetric            = "android_batt"
	AndroidCPUMetric                = "android_cpu"
	AndroidMemoryMetric             = "android_mem"
	AndroidMemoryUnaggregatedMetric = "android_mem_unagg"
	AndroidPackageList              = "android_package_list"
	AndroidIonMetric                = "android_ion"
	AndroidFastrpcMetric            = "android_fastrpc"
	AndroidLmkMetric                = "android_lmk"
	AndroidPowerRails               = "android_powrails"
	AndroidStartupMetric            = "android_startup"
	TraceMetadata                   = "trace_metadata"
	TraceAnalysisStats              = "trace_stats"
	UnsymbolizedFrames              = "unsymbolized_frames"
	JavaHeapStats                   = "java_heap_stats"
	JavaHeapHistogram               = "java_heap_histogram"
	AndroidLmkReasonMetric          = "android_lmk_reason"
	AndroidHwuiMetric               = "android_hwui_metric"
	AndroidDisplayMetrics           = "display_metrics"
	AndroidTaskNames                = "android_task_names"
	AndroidSurfaceflingerMetric     = "android_surfaceflinger"
	AndroidGpuMetric                = "android_gpu"
	AndroidSysUICujMetrics          = "android_sysui_cuj"
	AndroidJankCujMetric            = "android_jank_cuj"
	AndroidHwcomposerMetrics        = "android_hwcomposer"
	G2dMetrics                      = "g2d"
	AndroidDmaHeapMetric            = "android_dma_heap"
	AndroidTraceQualityMetric       = "android_trace_quality"
	ProfilerSmaps                   = "profiler_smaps"
	AndroidMultiuserMetric          = "android_multiuser"
	AndroidSimpleperfMetric         = "android_simpleperf"
	AndroidCameraMetric             = "android_camera"
	// AndroidDvfsMetric is the metrics for dynamic voltage and frequency scaling.
	AndroidDvfsMetric               = "android_dvfs"
	AndroidNetworkMetric            = "android_netperf"
	AndroidCameraUnaggregatedMetric = "android_camera_unagg"
	AndroidRtRuntimeMetric          = "android_rt_runtime"
	AndroidIrqRuntimeMetric         = "android_irq_runtime"
	AndroidTrustyWorkqueues         = "android_trusty_workqueues"
	AndroidOtherTracesMetric        = "android_other_traces"
	AndroidBinderMetric             = "android_binder"
	AndroidFrameTimelineMetric      = "android_frame_timeline_metric"
)

// ForceEnableTrace will overwrite tracing_on flag in kernel tracefs. In some cases this flag
// are occupied by unknown reason during ARC booting.
// Root permission is needed for this function. Failure may caused by permission issue or
// wrong debugfs path.
func ForceEnableTrace(ctx context.Context, a *arc.ARC) error {
	const (
		cmd = "echo 0 > "
		// TODO(sstan): Figure out different tracePath for R/T and x86/arm.
		// tracePath is the path of switcher in debugfs. The path may depend on the kernel version.
		tracePath = "/sys/kernel/tracing/tracing_on"
	)

	if err := a.Command(ctx, cmd+tracePath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to force enable trace")
	}
	return nil
}

// Trace will push the config from traceConfigPath to ARC device, start the perfetto
// basing on config, run the function, and pull the trace result from ARC device to
// traceResultPath. Note that if earlyExit is true, the perfetto tracing will be stopped
// after test function return.
func Trace(ctx context.Context, a *arc.ARC, traceConfigPath, traceResultPath string, earlyExit bool, f func(context.Context) error) error {
	// Perfetto related path inner ARC.
	const (
		localPerfettoTraceDir = "/data/misc/perfetto-traces/"
		localTempResultPath   = localPerfettoTraceDir + "perfetto.trace"
	)

	config, err := ioutil.ReadFile(traceConfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to read config file")
	}

	shellCmd := a.Command(ctx, "perfetto", "-o", localTempResultPath, "--txt", "--config", "-")
	shellCmd.Cmd.Stdin = bytes.NewReader(config)

	if err := shellCmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start perfetto trace")
	}
	defer shellCmd.Wait()

	ferr := f(ctx)

	// If earlyExit, stop tracing immediately. Or wait tracing finish.
	if earlyExit {
		shellCmd.Kill()
	} else {
		shellCmd.Wait(testexec.DumpLogOnError)
	}

	// Pull trace result whatever test function succeeded or failed.
	if err := a.PullFile(ctx, localTempResultPath, traceResultPath); err != nil {
		return errors.Wrapf(err, "failed to pull perfetto from ARC path %v to %v", localTempResultPath, traceResultPath)
	}

	if ferr != nil {
		return errors.Wrap(ferr, "finish trace but errors happen on test func")
	}

	return nil
}

// Metrics use the higher-level query interface that run pre-baked queries called metrics.
// They defined in Android-AOSP path: external/perfetto/protos/perfetto/metrics/android/
// It also support customized metrics, see https://perfetto.dev/docs/analysis/metrics
func Metrics(ctx context.Context, traceResultPath string, metrics ...string) (*perfetto_proto.TraceMetrics, error) {
	if err := ensureTraceProcessorExistence(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to initialize trace processor")
	}

	// Explicit use python since the tmp filesystem may be mounted with "noexec" option.
	output, err := testexec.CommandContext(ctx, "python", traceProcessorPath, "--run-metrics", strings.Join(metrics, ","), traceResultPath).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	metricsProto := &perfetto_proto.TraceMetrics{}
	if err := proto.UnmarshalText(string(output), metricsProto); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal metrics result")
	}

	return metricsProto, nil
}

func ensureTempDir() error {
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return errors.Wrap(err, "failed to create temp dir for perfetto")
	}
	return nil
}

func ensureTraceProcessorExistence(ctx context.Context) error {
	if err := ensureTempDir(); err != nil {
		// Returns err without wrap since it just try to create dir.
		return err
	}
	if _, err := os.Stat(traceProcessorPath); err == nil {
		// If trace processor already exist on DUT, just return.
		return nil
	} else if os.IsNotExist(err) {
		if err := testexec.CommandContext(ctx, "curl", "-o", traceProcessorPath, "-L", traceProcessorURL).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to download trace processor")
		}
		if err := testexec.CommandContext(ctx, "chmod", "+x", traceProcessorPath).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to chmod for trace processor")
		}
	} else {
		return errors.Wrap(err, "failed to ensure trace processer existence")
	}
	return nil
}
