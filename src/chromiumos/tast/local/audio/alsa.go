// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Example of the output from `aplay -l` when using 3 devices, 4 streams
/*
	**** List of PLAYBACK Hardware Devices ****
	card 0: SoundCard [VirtIO SoundCard], device 0: virtio-snd [VirtIO PCM 0]
	Subdevices: 4/4
	Subdevice #0: subdevice #0
	Subdevice #1: subdevice #1
	Subdevice #2: subdevice #2
	Subdevice #3: subdevice #3
	card 0: SoundCard [VirtIO SoundCard], device 1: virtio-snd [VirtIO PCM 1]
	Subdevices: 4/4
	  Subdevice #0: subdevice #0
	  Subdevice #1: subdevice #1
	  Subdevice #2: subdevice #2
	  Subdevice #3: subdevice #3
	  card 0: SoundCard [VirtIO SoundCard], device 2: virtio-snd [VirtIO PCM 2]
	  Subdevices: 4/4
	  Subdevice #0: subdevice #0
	  Subdevice #1: subdevice #1
	  Subdevice #2: subdevice #2
	  Subdevice #3: subdevice #3
*/

var deviceRegex = regexp.MustCompile(`card \d+:.*\[(?P<Card>.*)\], device \d+:.*\[(?P<Device>.*)\]`)
var subDeviceCountRegex = regexp.MustCompile(`Subdevices: \d+/\d+`)

// CheckAlsaDeviceList checks whether the output of aplay -l or arecord -l matches
// the expectation and return an error on the first difference.
func CheckAlsaDeviceList(
	ctx context.Context,
	stdout string,
	expectedCardNames,
	expectedDeviceNames []string,
	expectedStreams int,
	strictSubdeviceCount bool,
) error {
	testing.ContextLog(ctx, stdout)
	lines := strings.Split(stdout, "\n")
	devicesCnt := 0
	for idx := 0; idx < len(lines); idx++ {
		lines[idx] = strings.TrimSpace(lines[idx])

		match := deviceRegex.FindStringSubmatch(lines[idx])
		if match == nil {
			continue
		}

		if devicesCnt >= len(expectedCardNames) {
			return errors.Errorf("card name count is more than expected: got %s", match[1])
		}

		if devicesCnt >= len(expectedDeviceNames) {
			return errors.Errorf("device name count is more than expected: got %s", match[2])
		}

		if match[1] != expectedCardNames[devicesCnt] {
			return errors.Errorf(
				"card name incorrect: got %s, want %s",
				match[1], expectedCardNames[devicesCnt],
			)
		}
		if match[2] != expectedDeviceNames[devicesCnt] {
			return errors.Errorf(
				"device name incorrect: got %s, want %s",
				match[2], expectedDeviceNames[devicesCnt],
			)
		}
		devicesCnt++

		// Expect next line: "Subdevices: n/n"
		idx++
		if idx >= len(lines) {
			return errors.Errorf("device %s has no subdevices line after it", match[2])
		}

		lines[idx] = strings.TrimSpace(lines[idx])

		if strictSubdeviceCount {
			expectSubdevices := fmt.Sprintf("Subdevices: %d/%d", expectedStreams, expectedStreams)
			if lines[idx] != expectSubdevices {
				return errors.Errorf(
					"device %s subdevices line incorrect: got %q, want %q",
					match[2], lines[idx], expectSubdevices,
				)
			}
		} else {
			if subDeviceCountRegex.FindStringSubmatch(lines[idx]) == nil {
				return errors.Errorf(
					"device %s subdevices line incorrect: got %q",
					match[2], lines[idx],
				)
			}
		}
	}

	if devicesCnt != len(expectedDeviceNames) {
		return errors.Errorf(
			"device count incorrect: got %d, want %d",
			devicesCnt, len(expectedDeviceNames),
		)
	}

	return nil
}
