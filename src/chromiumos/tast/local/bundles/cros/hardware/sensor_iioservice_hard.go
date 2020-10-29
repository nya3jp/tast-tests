// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	nDuration        = 10 // seconds
	nClientPerSensor = 3
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorIioserviceHard,
		Desc: "Tests that iioservice provides sensors' samples properly",
		Contacts: []string{
			"gwendal@chromium.com",      // Chrome OS sensors point of contact
			"chenghaoyang@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// SensorIioserviceHard reads all devices' samples from daemon iioservice.
func SensorIioserviceHard(ctx context.Context, s *testing.State) {
	sensors, err := iio.GetSensors()
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	var wg sync.WaitGroup

	timestampRegex := regexp.MustCompile(`timestamp: (?P<ts>\d+)`)
	errorRegex := regexp.MustCompile(".*:ERROR:.*")

	errorCh := make(chan error, 1000)
	for i := 0; i < nClientPerSensor; i++ {
		for _, sn := range sensors {
			if sn.Name == iio.Light {
				continue
			}
			wg.Add(1)
			go func(wg *sync.WaitGroup, ctx context.Context, sn *iio.Sensor, i int) {
				defer wg.Done()

				start, err := iio.BootTime()
				if err != nil {
					errorCh <- errors.Wrap(err, "error reading BootTime: ")
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
					fmt.Sprintf("--samples=%d", nDuration*sn.MaxFrequency/1000))
				cmdStr := shutil.EscapeSlice(cmd.Args)
				pipe, err := cmd.StderrPipe()
				if err != nil {
					errorCh <- errors.Wrapf(err, "failed to obtain a pipe for %s -%d", cmdStr, i)
					return
				}
				if err := cmd.Start(); err != nil {
					errorCh <- errors.Wrapf(err, "failed to start %s", cmdStr)
					return
				}
				var rs []*iio.SensorReading
				scanner := bufio.NewScanner(pipe)
				for scanner.Scan() {
					l := scanner.Text()

					rawTs := timestampRegex.FindStringSubmatch(l)
					if rawTs != nil {
						var r iio.SensorReading
						ts, err := strconv.ParseInt(rawTs[1], 10, 64)
						if err != nil {
							errorCh <- errors.Errorf("%d: invalid timestmap %s", sn.IioID, rawTs)
							return
						}
						r.Timestamp = time.Duration(ts)
						rs = append(rs, &r)
					}

					if errorRegex.MatchString(l) {
						errorCh <- errors.Errorf("%d(%d): %s", sn.IioID, i, l)
					}
				}

				if err := scanner.Err(); err != nil {
					errorCh <- errors.Wrap(err, "error while scanning simpleclient")
					return
				}

				if err := cmd.Wait(testexec.DumpLogOnError); err != nil {
					errorCh <- errors.Wrapf(err, "failed to wait for exit: %s", cmdStr)
				}

				end, err := iio.BootTime()
				if err != nil {
					errorCh <- errors.Wrap(err, "error reading BootTime: ")
					return
				}
				s.Logf("Got %v readings from %v %v",
					len(rs), sn.Location, sn.Name)
				const collectTime = nDuration * time.Second
				errorCh <- iio.Validate(rs, start, end, sn, collectTime)
				return
			}(&wg, ctx, sn, i)
		}
	}
	/* Wait for all clients to close. */
	wg.Wait()

	for {
		err := <-errorCh
		if err == nil {
			break
		} else {
			s.Error(" : ", err)
		}
	}
}
