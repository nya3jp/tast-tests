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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorIIOServiceHard,
		Desc: "Tests that iioservice provides sensors' samples properly",
		Contacts: []string{
			"gwendal@chromium.com",      // Chrome OS sensors point of contact
			"chenghaoyang@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// SensorIIOServiceHard reads all devices' samples from daemon iioservice.
func SensorIIOServiceHard(ctx context.Context, s *testing.State) {
	const (
		nDuration        = 10 * time.Second
		nClientPerSensor = 3
	)

	sensors, err := iio.GetSensors()
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	timestampRegex := regexp.MustCompile(`timestamp: (?P<ts>\d+)`)
	errorRegex := regexp.MustCompile(".*:ERROR:.*")

	errorCh := make(chan error)
	numTasks := 0
	for i := 0; i < nClientPerSensor; i++ {
		for _, sn := range sensors {
			if sn.Name == iio.Light {
				continue
			}
			numTasks++
			go func(ctx context.Context, sn *iio.Sensor, i int) {
				// Return only one error.
				start, err := iio.BootTime()
				if err != nil {
					errorCh <- errors.Wrap(err, "error reading BootTime: ")
					return
				}
				channels := "timestamp"
				if sn.Name == iio.Accel {
					channels += " accel_x accel_y accel_z"
				} else if sn.Name == iio.Gyro {
					channels += " anglvel_x anglvel_y anglvel_z"
				}
				cmd := testexec.CommandContext(
					ctx, "iioservice_simpleclient", fmt.Sprintf("--frequency=%d.%03d",
						sn.MaxFrequency/1000, sn.MaxFrequency%1000), fmt.Sprintf("--timeout=%d", int(nDuration.Milliseconds()*2)),
					"--channels="+channels, fmt.Sprintf("--device_id=%d", sn.IioID),
					fmt.Sprintf("--samples=%d", int(nDuration.Seconds()*float64(sn.MaxFrequency)/1000)))
				cmdStr := shutil.EscapeSlice(cmd.Args)
				_, stderr, err := cmd.SeparatedOutput()
				if err != nil {
					errorCh <- errors.Wrapf(err, "failed to run %s", cmdStr)
					return
				}
				var rs []*iio.SensorReading
				var errorLines []string
				for _, l := range strings.Split(string(stderr), "\n") {
					rawTs := timestampRegex.FindStringSubmatch(l)
					if rawTs != nil {
						ts, err := strconv.ParseInt(rawTs[1], 10, 64)
						if err != nil {
							errorCh <- errors.Errorf("%d: invalid timestmap %s", sn.IioID, rawTs)
							return
						}

						var r iio.SensorReading
						r.Timestamp = time.Duration(ts)
						rs = append(rs, &r)
					}

					if errorRegex.MatchString(l) {
						errorLines = append(errorLines, l)
					}
				}

				if len(errorLines) != 0 {
					errorCh <- errors.Errorf("%d(%d): %s", sn.IioID, i, strings.Join(errorLines, "\n"))
					return
				}

				end, err := iio.BootTime()
				if err != nil {
					errorCh <- errors.Wrap(err, "error reading BootTime: ")
					return
				}
				s.Logf("Got %v readings from %v %v",
					len(rs), sn.Location, sn.Name)
				if err := iio.Validate(rs, start, end, sn, nDuration); err != nil {
					errorCh <- errors.Wrap(err, "error during validation: ")
					return
				}
				errorCh <- nil
			}(ctx, sn, i)
		}
	}

	// Wait a single error message from each tasks.
	for i := 0; i < numTasks; i++ {
		if err := <-errorCh; err != nil {
			s.Error(" : ", err)
		}
	}
}
