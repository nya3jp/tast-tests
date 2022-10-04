// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import "math"

// GetFrequencyFromData get frequency from recorded audio data.
// The method used here is the same as `sox stat` rough frequency.
func GetFrequencyFromData(data []float64, sampleRate float64) float64 {
	var dsum2, sum2, last float64
	for i := range data {
		samp := data[i] / 2147483647.0 // Scale down
		if i == 0 {
			last = samp
		}
		sum2 += samp * samp
		delta := samp - last
		dsum2 += delta * delta
		last = samp
	}
	if sum2 == 0 {
		// Return -1 if all data are zeros, because we could not calculate frequency.
		return -1
	}
	return math.Sqrt(dsum2/sum2) * sampleRate / (math.Pi * 2)
}
