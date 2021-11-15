// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"fmt"
	"strings"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/metrics/github.com/google/perfetto/perfetto_proto"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/tracing"
	"chromiumos/tast/testing"
)

const onErrorOccurred = "OnErrorOccurred:"
const latencyExceedsTolerance = "Max latency exceeds latency tolerance."
const succeedReadingSamples = "Number of success reads"

const traceMetricCPU = "android_cpu"
const targetProcessName = "/usr/sbin/iioservice"

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorIioservice,
		Desc: "Tests that iioservice provides sensors' samples properly",
		Contacts: []string{
			"gwendal@chromium.com",      // Chrome OS sensors point of contact
			"chenghaoyang@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		Data:         []string{tracing.TBMTracedProbesConfigFile, tracing.TraceProcessor()},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"iioservice"},
	})
}

// processCPUMetric extracts information of the target process in the
// cpu metric.
func processCPUMetric(cpuMetric *perfetto_proto.AndroidCpuMetric, s *testing.State) {
	foundTarget := false
	for _, processInfo := range cpuMetric.GetProcessInfo() {
		if processInfo.GetName() == targetProcessName {
			foundTarget = true

			metric := processInfo.GetMetrics()
			s.Log("megacycles: ", metric.GetMcycles())
			s.Log("runtime in nanosecond: ", metric.GetRuntimeNs())
			s.Log("min_freq in kHz: ", metric.GetMinFreqKhz())
			s.Log("max_freq in kHz: ", metric.GetMaxFreqKhz())
			s.Log("avg_freq in kHz: ", metric.GetAvgFreqKhz())

			break
		}
	}

	if foundTarget == false {
		s.Error("Failed to find the target process: ", targetProcessName)
	}
}

// SensorIioservice reads all devices' samples from daemon iioservice.
func SensorIioservice(ctx context.Context, s *testing.State) {
	var maxFreq int
	var strOut string

	// Call libmems' functions directly here to read and verify samples
	sensors, err := iio.GetSensors(ctx)
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	for _, sn := range sensors {
		// Start a trace session using the perfetto command line tool.
		traceConfigPath := s.DataPath(tracing.TBMTracedProbesConfigFile)
		sess, err := tracing.StartSession(ctx, traceConfigPath)
		// The temporary file of trace data is no longer needed when returned.
		defer sess.RemoveTraceResultFile()

		maxFreq = sn.MaxFrequency

		if sn.Name == iio.Ring {
			s.Error("Kernel must be compiled with USE=iioservice")
		}

		if sn.Name == iio.Activity || sn.Name == iio.Light {
			continue
		}

		frequency := fmt.Sprintf("--frequency=%f", float64(maxFreq)/1000)

		out, err := testexec.CommandContext(ctx, "iioservice_simpleclient",
			fmt.Sprintf("--device_id=%d", sn.IioID), "--channels=timestamp",
			frequency).CombinedOutput()

		if err != nil {
			s.Error("Error reading samples on DUT: ", err)
		}

		strOut = string(out)
		if strings.Contains(strOut, onErrorOccurred) {
			s.Error("OnErrorOccurred: ", sn.Name)
		} else if strings.Contains(strOut, latencyExceedsTolerance) {
			s.Error("Latency Exceeds Tolerance: ", sn.Name)
		} else if !strings.Contains(strOut, succeedReadingSamples) {
			s.Error("Not enough successful readsamples on sensor: ", sn.Name)
		} else {
			s.Logf("Test passed on device name: %v, id: %v", sn.Name, sn.IioID)
		}

		if err := sess.Stop(); err != nil {
			s.Fatal("Failed to stop the tracing session: ", err)
		}

		metrics, err := sess.RunMetrics(ctx, s.DataPath(tracing.TraceProcessor()), []string{traceMetricCPU})
		if err != nil {
			s.Fatal("Failed to RunMetrics: ", err)
		}

		processCPUMetric(metrics.GetAndroidCpu(), s)
	}
}
