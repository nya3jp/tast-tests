// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package soundcardinit

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TestParameters holds the  test parameters.
type TestParameters struct {
	SoundCardID string
	AmpCount    uint
	Func        func(context.Context, *testing.State, string)
}

// StopTimeFile is the file stores previous CRAS stop time.
const StopTimeFile = "/var/lib/cras/stop"

// RunTimeFile is the file stores previous sound_card_init run time.
const RunTimeFile = "/var/lib/sound_card_init/%s/run"

// CalibFiles is the file stores previous calibration values.
const CalibFiles = "/var/lib/sound_card_init/%s/calib_%d"

const timestampYamlFile = `
---
secs: %d
nanos: 0
`

// RemoveCalibFiles removes CalibFiles.
func RemoveCalibFiles(ctx context.Context, soundCardID string, count uint) error {
	for i := 0; i < int(count); i++ {
		calib := fmt.Sprintf(CalibFiles, soundCardID, i)
		if err := os.Remove(calib); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to rm %s: ", calib)
		}
	}
	return nil
}

// CreateRunTimeFile create a RunTimeFile containing given unix time in secs.
func CreateRunTimeFile(ctx context.Context, soundCardID string, ts int64) error {
	s := fmt.Sprintf(timestampYamlFile, ts)
	f := fmt.Sprintf(RunTimeFile, soundCardID)
	return ioutil.WriteFile(f, []byte(s), 0644)
}

// CreateStopTimeFile create a StopTimeFile containing given unix time in secs.
func CreateStopTimeFile(ctx context.Context, ts int64) error {
	s := fmt.Sprintf(timestampYamlFile, ts)
	return ioutil.WriteFile(StopTimeFile, []byte(s), 0644)
}

// VerifyUseVPD verifies calib* content contains UseVPD.
func VerifyUseVPD(ctx context.Context, soundCardID string, count uint) error {
	for i := 0; i < int(count); i++ {
		calib := fmt.Sprintf(CalibFiles, soundCardID, i)
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
func VerifyCalibNotExist(ctx context.Context, soundCardID string, count uint) error {
	for i := 0; i < int(count); i++ {
		calib := fmt.Sprintf(CalibFiles, soundCardID, i)
		if _, err := os.Stat(calib); os.IsNotExist(err) {
			continue
		} else {
			return errors.Errorf("expect %s does not exist", calib)
		}
	}
	return nil
}

// GetSoundCardID retrieves sound card name by parsing aplay-l output.
// An example of "aplay -l" log is shown as below:
// **** List of PLAYBACK Hardware Devices ****
// card 0: sofcmlmax98390d [sof-cml_max98390_da7219], device 0: Speakers (*) []
//  Subdevices: 1/1
//  Subdevice #0: subdevice #0
func GetSoundCardID(ctx context.Context) (string, error) {
	re := regexp.MustCompile(`card 0: [a-z,0-9]+ `)
	var soundCardID string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		dump, err := testexec.CommandContext(ctx, "aplay", "-l").Output()
		if err != nil {
			return testing.PollBreak(errors.Errorf("failed to aplay -l: %s", err))
		}
		str := re.FindString(string(dump))
		if str == "" {
			return errors.New("no sound card")
		}
		soundCardID = strings.Trim(strings.TrimLeft(str, "card 0: "), " ")
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return "", err
	}
	return soundCardID, nil
}
