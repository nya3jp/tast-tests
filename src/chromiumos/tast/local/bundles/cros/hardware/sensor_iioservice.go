// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/bundles/cros/hardware/util"
	"chromiumos/tast/local/tracing"
	"chromiumos/tast/testing"
)

const onErrorOccurred = "OnErrorOccurred:"
const latencyExceedsTolerance = "Max latency exceeds latency tolerance."
const succeedReadingSamples = "Number of success reads"

var latencies = map[string]string{
	"max":    "Max latency  ",
	"min":    "Min latency  ",
	"median": "Median latency  ",
	"mean":   "Mean latency  ",
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorIioservice,
		Desc: "Tests that iioservice provides sensors' samples properly",
		Contacts: []string{
			"gwendal@chromium.com",      // Chrome OS sensors point of contact
			"chenghaoyang@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		Data:         []string{tracing.TBMTracedProbesConfigFile, tracing.TraceProcessorAmd64, tracing.TraceProcessorArm, tracing.TraceProcessorArm64},
		Attr:         []string{"group:mainline", "informational", "group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"iioservice"},
	})
}

func processLatency(strOut string, sensor *iio.Sensor, pv *perf.Values, s *testing.State) error {
	re := regexp.MustCompile(`[+-]?([0-9]*[.])?[0-9]+`)

	for latency, pattern := range latencies {
		index := strings.Index(strOut, pattern)
		if index == -1 {
			return errors.New("Failed to find latency pattern: " + pattern)
		}

		value, err := strconv.ParseFloat(re.FindString(strOut[index:]), 64)
		if err != nil {
			return errors.Wrap(err, "failed to parse latency to float")
		}

		pv.Set(perf.Metric{
			Name:      "SensorIioservice." + string(sensor.Name) + "." + string(sensor.Location) + ".latency." + latency,
			Unit:      "second",
			Direction: perf.SmallerIsBetter,
		}, value)
	}

	return nil
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

		metrics, err := sess.RunMetrics(ctx, s.DataPath(tracing.TraceProcessor()), []string{util.TraceMetricCPU, util.TraceMetricMEM})
		if err != nil {
			s.Fatal("Failed to RunMetrics: ", err)
		}

		pv := perf.NewValues()

		if err := processLatency(strOut, sn, pv, s); err != nil {
			s.Error("Failed to process latency: ", err)
		}

		// As there's no existing metrics for ChromeOS, and Android ones are quite useful, use the existing Android metrics for now.
		if err := util.ProcessCPUMetric(ctx, metrics.GetAndroidCpu(), "SensorIioservice", util.IioserviceProcessName, pv); err != nil {
			s.Fatal("Failed to process CPU metric: ", err)
		}
		if err := util.ProcessMemMetric(ctx, metrics.GetAndroidMem(), "SensorIioservice", util.IioserviceProcessName, pv); err != nil {
			s.Fatal("Failed to process memory metric: ", err)
		}

		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}
}
