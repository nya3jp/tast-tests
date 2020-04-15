// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio interacts with audio operation.
package audio

import (
	"context"
	"math"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ConvertRawToWav converts the audio raw file to wav file. The params are the default setting of cras_test_client.
func ConvertRawToWav(ctx context.Context, rawFileName, wavFileName string) error {
	err := testexec.CommandContext(ctx, "sox", "-b", "16", "-r", "48000", "-c", "2", "-e", "signed", "-t", "raw", rawFileName, wavFileName).Run(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "sox failed")
	}
	return nil
}

// TrimFileFrom removes first x seconds from the file.
func TrimFileFrom(ctx context.Context, oldFileName, newFileName string, fromSeconds float64) error {
	err := testexec.CommandContext(
		ctx, "sox", oldFileName, newFileName, "trim",
		strconv.FormatFloat(fromSeconds, 'f', -1, 64)).Run(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "sox failed")
	}
	return nil
}

// CheckRecordingQuality checks the recording file to see whether internal mic works normally.
// A qualified file must meet these requirements:
// 1. The RMS must small than the threshold. If not, it may be the static noise inside.
// 2. The recorded samples can not be all zeros. It is impossible for a normal internal mic.
func CheckRecordingQuality(ctx context.Context, fileName string) error {

	const (
		threshold = -6.0
	)

	out, err := testexec.CommandContext(ctx, "sox", fileName, "-n", "stats").CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "sox failed")
	}
	re := regexp.MustCompile("RMS Pk dB +\\S+ +(\\S+) +(\\S+)")
	rms := re.FindStringSubmatch(string(out))

	rms1, err := strconv.ParseFloat(rms[1], 32)
	if err != nil {
		return errors.Wrap(err, "atof failed")
	}
	rms2, err := strconv.ParseFloat(rms[2], 32)
	if err != nil {
		return errors.Wrap(err, "atof failed")
	}

	testing.ContextLogf(ctx, "Left channel RMS: %f dB", rms1)
	testing.ContextLogf(ctx, "Right channel RMS: %f dB", rms2)

	if rms1 > threshold || rms2 > threshold {
		return errors.New("the RMS is too large")
	}
	// If all samples are zeros, the rms is -inf.
	if math.IsInf(rms1, -1) || math.IsInf(rms2, -1) {
		return errors.New("the RMS is too small")
	}

	return nil
}
