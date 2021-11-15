// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/bundles/cros/hardware/util"
	"chromiumos/tast/local/tracing"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const targetProcessName = "/usr/sbin/iioservice"

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorIioserviceHard,
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

func runSingleClient(ctx context.Context, s *testing.State, sn *iio.Sensor, i int) error {
	const nDuration = 10 * time.Second

	timestampRegex := regexp.MustCompile(`timestamp: (?P<ts>\d+)`)
	errorRegex := regexp.MustCompile(".*:ERROR:.*")

	// Return only one error.
	start, err := iio.BootTime()
	if err != nil {
		return errors.Wrap(err, "error reading BootTime")
	}
	channels := "timestamp"
	if sn.Name == iio.Accel {
		channels += " accel_x accel_y accel_z"
	} else if sn.Name == iio.Gyro {
		channels += " anglvel_x anglvel_y anglvel_z"
	} else if sn.Name == iio.Mag {
		channels += " magn_x magn_y magn_z"
	} else if sn.Name == iio.Ring {
		return errors.New("Kernel must be compiled with USE=iioservice")
	} else {
		// This sensor is not supported by this test, skip.
		return nil
	}
	cmd := testexec.CommandContext(
		ctx, "iioservice_simpleclient", fmt.Sprintf("--frequency=%d.%03d",
			sn.MaxFrequency/1000, sn.MaxFrequency%1000), fmt.Sprintf("--timeout=%d", int(nDuration.Milliseconds()*2)),
		"--channels="+channels, fmt.Sprintf("--device_id=%d", sn.IioID),
		fmt.Sprintf("--samples=%d", int(nDuration.Seconds()*float64(sn.MaxFrequency)/1000)))

	_, stderr, err := cmd.SeparatedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to run %s", shutil.EscapeSlice(cmd.Args))
	}
	var rs []*iio.SensorReading
	lastErrorFound := ""
	for _, l := range strings.Split(string(stderr), "\n") {
		rawTs := timestampRegex.FindStringSubmatch(l)
		if rawTs != nil {
			ts, err := strconv.ParseInt(rawTs[1], 10, 64)
			if err != nil {
				return errors.Errorf("%d: invalid timestmap %s", sn.IioID, rawTs)
			}

			var r iio.SensorReading
			r.Timestamp = time.Duration(ts)
			rs = append(rs, &r)
		}

		if errorRegex.MatchString(l) {
			lastErrorFound = l
			s.Logf("%s %d(%d): %s", sn.Name, sn.IioID, i, lastErrorFound)
		}
	}

	if lastErrorFound != "" {
		return errors.Errorf("%s %d(%d): failed last error: %s", sn.Name, sn.IioID, i, lastErrorFound)
	}

	end, err := iio.BootTime()
	if err != nil {
		return errors.Wrap(err, "error reading BootTime")
	}
	s.Logf("Got %v readings from %v %v",
		len(rs), sn.Location, sn.Name)
	if err := iio.Validate(rs, start, end, sn, nDuration); err != nil {
		return errors.Wrap(err, "error during validation")
	}
	return nil
}

// SensorIioserviceHard reads all devices' samples from daemon iioservice.
func SensorIioserviceHard(ctx context.Context, s *testing.State) {
	const nClientPerSensor = 3

	sensors, err := iio.GetSensors(ctx)
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	// Start a trace session using the perfetto command line tool.
	traceConfigPath := s.DataPath(tracing.TBMTracedProbesConfigFile)
	sess, err := tracing.StartSession(ctx, traceConfigPath)
	// The temporary file of trace data is no longer needed when returned.
	defer sess.RemoveTraceResultFile()

	errorCh := make(chan error)
	numTasks := 0
	for i := 0; i < nClientPerSensor; i++ {
		for _, sn := range sensors {
			if sn.Name == iio.Light {
				continue
			}
			numTasks++
			go func(ctx context.Context, s *testing.State, sn *iio.Sensor, i int) {
				if err := runSingleClient(ctx, s, sn, i); err != nil {
					errorCh <- errors.Wrapf(err, "Client %d to %s failed: ", i, sn.Name)
				} else {
					errorCh <- nil
				}
			}(ctx, s, sn, i)
		}
	}

	// Wait a single error message from each tasks.
	for i := 0; i < numTasks; i++ {
		if err := <-errorCh; err != nil {
			s.Error(" : ", err)
		}
	}

	if err := sess.Stop(); err != nil {
		s.Fatal("Failed to stop the tracing session: ", err)
	}

	metrics, err := sess.RunMetrics(ctx, s.DataPath(tracing.TraceProcessor()), []string{util.TraceMetricCPU, util.TraceMetricMEM})
	if err != nil {
		s.Fatal("Failed to RunMetrics: ", err)
	}

	pv := perf.NewValues()
	// As there's no existing metrics for ChromeOS, and Android ones are quite useful, use the existing Android metrics for now.
	if err := util.ProcessCPUMetric(metrics.GetAndroidCpu(), "SensorIioserviceHard", util.IioserviceProcessName, pv); err != nil {
		s.Fatal("Failed to process CPU metric: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}
