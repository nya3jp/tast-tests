// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	nDuration        = 5 // seconds
	nClientPerSensor = 2
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorService,
		Desc: "Tests that all sensors can be service by iioservice at their max frequency",
		Contacts: []string{
			"gwendal@chromium.com", // Chrome OS sensors point of contact
			"mathewk@chromium.org", // Test author
			"chromeos-sensors-eng@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func SensorService(ctx context.Context, s *testing.State) {
	dutSensors, err := iio.GetSensors()
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	ch := make(chan error)
	for i := 0; i < nClientPerSensor; i++ {
		for _, sn := range dutSensors {
			go func(ctx context.Context, sn *iio.Sensor) {
				start, err := iio.BootTime()
				if err != nil {
					ch <- errors.Wrap(err, "error reading BootTime: ")
					return
				}
				channels := "timestamp"
				maxChannels := 1
				if sn.Name == iio.Accel {
					channels += " accel_x accel_y accel_z"
					maxChannels += 3
				} else if sn.Name == iio.Gyro {
					channels += " anglvel_x anglvel_y anglvel_z"
					maxChannels += 3
				}
				cmd := testexec.CommandContext(
					ctx, "iioservice_simpleclient", fmt.Sprintf("--frequency=%d.%03d",
						sn.MaxFrequency/1000, sn.MaxFrequency%1000), fmt.Sprintf("--timeout=%d", 2*nDuration*1000),
					"--channels="+channels, fmt.Sprintf("--device_id=%d", sn.IioID),
					"--log_level=1", fmt.Sprintf("--samples=%d", nDuration*sn.MaxFrequency/1000))
				cmdStr := shutil.EscapeSlice(cmd.Args)
				pipe, err := cmd.StdoutPipe()
				if err != nil {
					ch <- errors.Wrapf(err, "failed to obtain a pipe for %s -%d", cmdStr, i)
					return
				}
				if err := cmd.Start(); err != nil {
					ch <- errors.Wrapf(err, "failed to start %s", cmdStr)
					return
				}
				var rs []*iio.SensorReading
				scanner := bufio.NewScanner(pipe)
				for scanner.Scan() {
					l := scanner.Text()

					var r iio.SensorReading

					input := strings.Split(l, "\t")
					if len(input) != maxChannels {
						ch <- errors.Errorf("invalid sample %s got %d elements", l, len(input))
						return
					}
					ts, err := strconv.ParseInt(input[maxChannels-1], 10, 64)
					if err != nil {
						ch <- errors.Errorf("invalid timestmap %s", input[maxChannels-1])
						return
					}

					r.Timestamp = time.Duration(ts)
					rs = append(rs, &r)
				}
				if err := scanner.Err(); err != nil {
					ch <- errors.Wrap(err, "error while scanning simpleclient")
					return
				}

				if err := cmd.Wait(testexec.DumpLogOnError); err != nil {
					ch <- errors.Wrap(err, "failed to wait for exit: ")
				}

				end, err := iio.BootTime()
				if err != nil {
					ch <- errors.Wrap(err, "error reading BootTime: ")
					return
				}
				s.Logf("Got %v readings from %v %v",
					len(rs), sn.Location, sn.Name)
				const collectTime = nDuration * time.Second
				ch <- iio.Validate(rs, start, end, sn, collectTime)
				return
			}(ctx, sn)
		}
	}
	for i := 0; i < nClientPerSensor; i++ {
		for _, sn := range dutSensors {
			if err := <-ch; err != nil {
				s.Errorf("check failed on sensor %v(%v): %v", sn.IioID, i, err)
			}
		}
	}

}
