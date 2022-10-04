// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"math"
	"testing"
)

// TestGetFrequencyFromData tests GetFrequencyFromData by generating sine wave with various
// parameters and ensure that the result is within the defined tolerance.
func TestGetFrequencyFromData(t *testing.T) {
	const (
		tolerance = 10
	)

	// Test parameters
	var (
		sampleRates = []int{16000, 44100, 48000}
		samples     = 1000
		amplitudes  = []float64{1, 256, 1000, 32768, 2147483647}

		freqMin  = 100
		freqMax  = 500
		freqStep = 5

		shiftsPerFreq = 10
	)

	data := make([]float64, samples)
	for _, sampleRate := range sampleRates {
		for _, amplitude := range amplitudes {
			for freq := freqMin; freq <= freqMax; freq += freqStep {
				for shift := 0; shift < shiftsPerFreq; shift++ {
					tShift := float64(sampleRate) / float64(freq) * float64(shift) / float64(shiftsPerFreq)
					// Generate sine wave
					for t := 0; t < samples; t++ {
						data[t] = float64(amplitude) * math.Sin(2*math.Pi*float64(freq)*(float64(t)+tShift)/float64(sampleRate))
					}

					// Analyse frequency
					res := GetFrequencyFromData(data, float64(sampleRate))
					if math.Abs(float64(freq)-res) > tolerance {
						t.Fatalf(
							"Result differs too much. sampleRate:%d amplitude:%f freq:%d, tShift:%f - got:%.2f, diff:%.2f",
							sampleRate,
							amplitude,
							freq,
							tShift,
							res,
							math.Abs(float64(freq)-res),
						)
					}
				}
			}
		}
	}
}

// TestGetFrequencyFromDataAllZeros tests the special case for GetFrequencyFromData where
// all data are zeros.
func TestGetFrequencyFromDataAllZeros(t *testing.T) {
	data := make([]float64, 1000)
	freq := GetFrequencyFromData(data, 48000)
	if math.Abs(freq - -1) > 1e-12 {
		t.Fatalf("Result incorrect. expect:-1, got:%f", freq)
	}
}
