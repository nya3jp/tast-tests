// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"io/ioutil"
	"math"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LoopbackRecord,
		Desc: "Checks that audio can be played and accurately captured over loopback",
		Contacts: []string{
			"cychiang@chromium.org",  // Media team
			"fletcherw@chromium.org", // Test author
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"audio_play", "audio_record"},
	})
}

func min16(a, b int16) int16 {
	if b < a {
		return b
	}
	return a
}

func max16(a, b int16) int16 {
	if b > a {
		return b
	}
	return a
}

func min(a, b int) int {
	if b < a {
		return b
	}
	return a
}

func max(a, b int) int {
	if b > a {
		return b
	}
	return a
}

// Trim leading/trailing zeroes from audio data.
func trimAudioData(samples []int16) []int16 {
	startIndex := 0
	for startIndex < len(samples) && samples[startIndex] == 0 {
		startIndex++
	}

	endIndex := len(samples) - 1
	for endIndex >= startIndex && samples[endIndex] == 0 {
		endIndex--
	}

	return samples[startIndex:endIndex]
}

// Given a byte slice representing raw audio data stored in S16LE 2 channel
// format, convert it to a slice of L and R channel values.
func decodeRawAudio(input []byte) (leftChannel, rightChannel []int16, err error) {
	if len(input)%4 != 0 {
		return nil, nil, errors.New("input length must be divisible by 4")
	}

	numSamples := len(input) / 4

	leftChannel = make([]int16, numSamples)
	rightChannel = make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		leftChannel[i] = int16(input[4*i+1])<<8 + int16(input[4*i])
		rightChannel[i] = int16(input[4*i+3])<<8 + int16(input[4*i+2])
	}
	return leftChannel, rightChannel, nil
}

// Given some played audio data, as well as some captured audio data which may
// contain the played audio data, try all possible start offsets within the captured
// audio and return the smallest squared error between the played data and the
// overlapping segment of captured data.
func findBestSquaredError(played, captured []int16) (lowestDistance float64, bestOffset int, err error) {
	if len(captured) < len(played) {
		err = errors.New("captured audio data cannot be shorter than played audio data")
		return math.MaxFloat64, 0, err
	}

	maxOffset := len(captured) - len(played)
	lowestDistance = math.MaxFloat64
	bestOffset = 0
	for offset := 0; offset <= maxOffset; offset++ {
		distance := 0.0
		for i := 0; i < len(played); i++ {
			if distance > lowestDistance {
				break
			}
			p := played[i]
			c := captured[offset+i]
			delta := float64(max16(p, c) - min16(p, c))
			distance += delta * delta / 1.0e+6
		}
		if distance < lowestDistance {
			lowestDistance = distance
			bestOffset = offset
		}
	}
	return lowestDistance, bestOffset, nil
}

func checkSignalIsContinuous(s *testing.State, samples []int16) {
	for i := 0; i < len(samples)-1; i++ {
		y1 := samples[i]
		y2 := samples[i+1]
		delta := max16(y1, y2) - min16(y1, y2)
		// If this is true, we've found a jump in the sine wave.
		// A more reliable way would be to estimate the derivative
		// of the signal and look for places where it exceeds MaxInt16,
		// since d/dx sin(x) = cos(x) and -1 <= cos(x) <= 1 (1 here
		// being analogous to the largest S16 signal aka MaxInt16).
		if delta > 3000 {
			start := max(0, i-5)
			end := min(len(samples), i+6)
			for j := start; j < end; j++ {
				if j == i || j == i+1 {
					s.Log("--> ", samples[j])
				} else {
					s.Log("    ", samples[j])
				}
			}
			s.Fatal("Signal must be continuous")
		}
	}
}

func LoopbackRecord(ctx context.Context, s *testing.State) {
	// Prepare input file.
	playbackFile, err := ioutil.TempFile("", "loopback_input*.raw")
	if err != nil {
		s.Fatal("Failed to create a tempfile: ", err)
	}
	defer os.Remove(playbackFile.Name())
	if err = playbackFile.Close(); err != nil {
		s.Fatal("Failed to close a tempfile: ", err)
	}

	// Generate a 6 second input file of a 420hz sine wave, sampled at
	// 48000hz in stereo, signed 16-bit little-endian format.
	sox := testexec.CommandContext(ctx, "sox", "-r48000", "-n", "-b16", "-c2", playbackFile.Name(), "synth", "6", "sin", "420")
	err = sox.Run()
	if err != nil {
		sox.DumpLog(ctx)
		s.Fatal("Failed: ", err)
	}

	// Record from the Pre DSP Loopback.
	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to connect to CRAS: ", err)
	}

	crasNodes, err := cras.GetNodes(ctx)
	if err != nil {
		s.Fatal("Failed to obtain CRAS nodes: ", err)
	}

	for _, n := range crasNodes {
		if n.Type == "POST_MIX_LOOPBACK" {
			err := cras.SetActiveNode(ctx, n)
			if err != nil {
				s.Fatal("Failed to set input node to Pre DSP Loopback: ", err)
			}
			break
		}
	}

	// Prepare file to capture to.
	captureFile, err := ioutil.TempFile("", "loopback_capture*.raw")
	if err != nil {
		s.Fatal("Failed to create a tempfile: ", err)
	}
	defer os.Remove(captureFile.Name())
	if err = captureFile.Close(); err != nil {
		s.Fatal("Failed to close a tempfile: ", err)
	}

	// Start capturing from loopback.
	capture := testexec.CommandContext(ctx, "nice", "-20", "cras_tests", "capture", "-c2", "-r48000", "-f", captureFile.Name())
	err = capture.Start()
	if err != nil {
		capture.DumpLog(ctx)
		s.Fatal("Failed: ", err)
	}

	timeout := func(cmd *testexec.Cmd) {
		testing.Sleep(ctx, 8*time.Second)
		e := cmd.Process.Signal(os.Interrupt)
		if e != nil {
			s.Fatal("Failed to send SIGINT to process: ", e)
		}
	}
	go timeout(capture)

	// Play input file.
	playback := testexec.CommandContext(ctx, "cras_tests", "playback", "-c2", "-r48000", "-f", playbackFile.Name())
	err = playback.Run()
	if err != nil {
		playback.DumpLog(ctx)
		s.Fatal("Failed: ", err)
	}

	err = capture.Wait()
	status, success := testexec.GetWaitStatus(err)
	if !success && (!status.Signaled() || (status.Signal() != os.Interrupt)) {
		capture.DumpLog(ctx)
		s.Fatal("Failed: ", err)
	}

	input, err := ioutil.ReadFile(playbackFile.Name())
	if err != nil {
		s.Fatal("Failed to read playback file: ", err)
	}

	output, err := ioutil.ReadFile(captureFile.Name())
	if err != nil {
		s.Fatal("Failed to read capture file: ", err)
	}

	leftIn, rightIn, err := decodeRawAudio(input)
	leftOut, rightOut, err := decodeRawAudio(output)

	s.Log("Number of Input Samples: ", len(leftIn))
	s.Log("Number of Output Samples: ", len(leftOut))

	trimmedLeftOut := trimAudioData(leftOut)
	trimmedRightOut := trimAudioData(rightOut)

	checkSignalIsContinuous(s, trimmedLeftOut)
	checkSignalIsContinuous(s, trimmedRightOut)

	leftDistance, leftOffset, err := findBestSquaredError(leftIn, leftOut)
	if err != nil {
		s.Fatal("Failed to compare left channel input/output: ", err)
	}

	rightDistance, rightOffset, err := findBestSquaredError(rightIn, rightOut)
	if err != nil {
		s.Fatal("Failed to compare right channel input/output: ", err)
	}

	s.Log("Best Offset: ", leftOffset, ", ", rightOffset)
	s.Log("Lowest Distance: ", leftDistance, ", ", rightDistance)
}
