// Copyright 2021 The ChromiumOS Authors
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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorIioserviceHard,
		Desc: "Tests that iioservice provides sensors' samples properly",
		Contacts: []string{
			"gwendal@chromium.com",      // ChromeOS sensors point of contact
			"chenghaoyang@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		Attr:         []string{"group:sensors"},
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
}
