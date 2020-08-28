// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio interacts with audio operation.
package audio

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// TestRawData is used to specify parameters of the audio test data, which should be raw, signed, and little-endian.
type TestRawData struct {
	// Path specifies the file path of audio data.
	Path string
	// BitsPerSample specifies bits per data sample.
	BitsPerSample int
	// Channels specifies the channel count of audio data.
	Channels int
	// Rate specifies the sampling rate.
	Rate int
	// Frequencies specifies the frequency of each channel, whose length should be equal to Channels.
	// This is only used in the sine tone generation of sox.
	Frequencies []int
	// Volume specifies the volume scale of sox, e.g. 0.5 to scale volume by half. -1.0 to invert.
	// This is only used in the sine tone generation of sox.
	Volume float32
	// Duration specifies the duration of audio data in seconds.
	Duration int
}

// ConvertRawToWav converts the audio raw file to wav file.
func ConvertRawToWav(ctx context.Context, rawFileName, wavFileName string, rate, channels int) error {
	err := testexec.CommandContext(
		ctx, "sox", "-b", "16",
		"-r", strconv.Itoa(rate),
		"-c", strconv.Itoa(channels),
		"-e", "signed",
		"-t", "raw",
		rawFileName, wavFileName).Run(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "sox failed")
	}
	return nil
}

// TrimFileFrom removes all samples before startTime from the file.
func TrimFileFrom(ctx context.Context, oldFileName, newFileName string, startTime time.Duration) error {
	err := testexec.CommandContext(
		ctx, "sox", oldFileName, newFileName, "trim",
		strconv.FormatFloat(startTime.Seconds(), 'f', -1, 64)).Run(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "sox failed")
	}
	return nil
}

// CheckRecordingQuality checks the recording file to see whether internal mic works normally.
// A qualified file must meet these requirements:
// 1. The RMS must be smaller than the threshold. If not, it may be the static noise inside.
// 2. The recorded samples can not be all zeros. It is impossible for a normal internal mic.
func CheckRecordingQuality(ctx context.Context, fileName string) error {
	const threshold = -10.0 // dB

	out, err := testexec.CommandContext(ctx, "sox", fileName, "-n", "stats").CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "sox failed")
	}
	re := regexp.MustCompile("RMS Pk dB +\\S+ +(\\S+) +(\\S+)")
	rms := re.FindStringSubmatch(string(out))

	if rms == nil {
		testing.ContextLog(ctx, "sox stats: ", string(out))
		return errors.New("could not find RMS info from the sox result")
	}

	rmsLeft, err := strconv.ParseFloat(rms[1], 32)
	if err != nil {
		return errors.Wrap(err, "atof failed")
	}
	rmsRight, err := strconv.ParseFloat(rms[2], 32)
	if err != nil {
		return errors.Wrap(err, "atof failed")
	}

	testing.ContextLogf(ctx, "Left channel RMS: %f dB", rmsLeft)
	testing.ContextLogf(ctx, "Right channel RMS: %f dB", rmsRight)

	if rmsLeft > threshold || rmsRight > threshold {
		return errors.Errorf("the RMS (%f, %f) is too large", rmsLeft, rmsRight)
	}
	// If all samples are zeros, the rms is -inf.
	if math.IsInf(rmsLeft, -1) || math.IsInf(rmsRight, -1) {
		return errors.New("the samples are all zeros")
	}

	return nil
}

// GetRmsAmplitude gets signal RMS of testData by sox.
func GetRmsAmplitude(ctx context.Context, testData TestRawData) (float64, error) {
	out, err := testexec.CommandContext(
		ctx, "sox",
		"-b", strconv.Itoa(testData.BitsPerSample),
		"-c", strconv.Itoa(testData.Channels),
		"-r", strconv.Itoa(testData.Rate),
		"-e", "signed",
		"-t", "raw",
		testData.Path, "-n", "stat").CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		return 0.0, errors.Wrap(err, "sox failed")
	}

	re := regexp.MustCompile("RMS\\s+amplitude:\\s+(\\S+)")
	match := re.FindStringSubmatch(string(out))

	if match == nil {
		testing.ContextLog(ctx, "sox stat: ", string(out))
		return 0.0, errors.New("could not find RMS info from the sox result")
	}

	rms, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0.0, errors.Wrap(err, "atof failed")
	}

	return rms, nil
}

// GenerateTestRawData generates sine raw data by sox with specified parameters in testData, and stores in testData.Path.
func GenerateTestRawData(ctx context.Context, testData TestRawData) error {
	if len(testData.Frequencies) != testData.Channels {
		return errors.Errorf("frequencies should have %d value(s)", testData.Channels)
	}

	args := []string{
		"-n",
		"-b", strconv.Itoa(testData.BitsPerSample),
		"-c", strconv.Itoa(testData.Channels),
		"-r", strconv.Itoa(testData.Rate),
		"-e", "signed",
		"-t", "raw",
		testData.Path,
		"synth", strconv.Itoa(testData.Duration),
	}
	for _, f := range testData.Frequencies {
		args = append(args, "sine", strconv.Itoa(f))
	}
	args = append(args, "vol", fmt.Sprintf("%f", testData.Volume))

	err := testexec.CommandContext(ctx, "sox", args...).Run(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "sox failed")
	}
	return nil
}
