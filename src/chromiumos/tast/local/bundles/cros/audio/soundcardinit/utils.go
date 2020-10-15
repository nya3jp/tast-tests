// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package soundcardinit

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// TestParameters holds the  test parameters.
type TestParameters struct {
	SoundCardID string
	AmpCount    uint
}

// StopTimeFile is the file stores previous CRAS stop time.
const StopTimeFile = "/var/lib/cras/stop"

// BootTimeFile is the file stores previous boot time.
const BootTimeFile = "/var/lib/sound_card_init/sofcmlmax98390d/boot"

// CalibFiles is the file stores previous calibration values.
const CalibFiles = "/var/lib/sound_card_init/sofcmlmax98390d/calib_%d"
const duration = 1 * time.Second

// RemoveStopTimeFile removes StopTimeFile.
func RemoveStopTimeFile(ctx context.Context) error {
	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	return testexec.CommandContext(
		runCtx, "rm", "-f",
		StopTimeFile,
	).Run(testexec.DumpLogOnError)
}

// RemoveBootTimeFile removes BootTimeFile.
func RemoveBootTimeFile(ctx context.Context) error {
	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	return testexec.CommandContext(
		runCtx, "rm", "-f",
		BootTimeFile,
	).Run(testexec.DumpLogOnError)
}

// RemoveCalibFiles removes CalibFiles.
func RemoveCalibFiles(ctx context.Context, count uint) error {
	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	for i := 0; i < int(count); i++ {
		calib := fmt.Sprintf(CalibFiles)
		if err := testexec.CommandContext(
			runCtx, "rm", "-f",
			calib,
		).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to rm %s: ", calib)
		}
	}
	return nil
}

// CreateBootTimeFile create a BootTimeFile containing given timestamps.
func CreateBootTimeFile(ctx context.Context, ts int64) error {
	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	return testexec.CommandContext(
		runCtx, "echo", "---\nts: ", strconv.Itoa(int(ts)), ">",
		BootTimeFile,
	).Run(testexec.DumpLogOnError)
}

// CreateStopTimeFile create a StopTimeFile containing given timestamps.
func CreateStopTimeFile(ctx context.Context, ts int64) error {
	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	return testexec.CommandContext(
		runCtx, "echo", "---\nts: ", strconv.Itoa(int(ts)), ">",
		StopTimeFile,
	).Run(testexec.DumpLogOnError)
}

// VerifyUseVPD verifies calib* content contains UseVPD.
func VerifyUseVPD(ctx context.Context, count uint) error {
	for i := 0; i < int(count); i++ {
		calib := fmt.Sprintf(CalibFiles, i)
		b, err := ioutil.ReadFile(calib)
		if err != nil {
			errors.Wrapf(err, "failed to read %s: ", calib)
		}
		if !strings.Contains(string(b), "UseVPD") {
			return errors.Errorf("%s expect:UseVPD, got: %s", calib, string(b))
		}
	}
	return nil
}

// VerifyCalibNotExist verifies calib* does not exist.
func VerifyCalibNotExist(ctx context.Context, count uint) error {
	for i := 0; i < int(count); i++ {
		calib := fmt.Sprintf(CalibFiles, i)
		if _, err := os.Stat("/path/to/whatever"); os.IsNotExist(err) {
			continue
		} else {
			return errors.Errorf("expect %s does not exist", calib)
		}
	}
	return nil
}
