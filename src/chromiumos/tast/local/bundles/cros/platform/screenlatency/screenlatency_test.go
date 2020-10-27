// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenlatency

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func AssertEqualLag(t *testing.T, expected, actual []time.Duration) {
	t.Helper()
	if len(expected) == 0 && len(actual) == 0 {
		// reflect.DeepEqual doesn't consider two empty slices to be equal.
		return
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func generateHostData(perFrameLines [][]string) hostData {
	ocrData := hostData{
		FramesMetaData:                     []frameData{},
		VideoFPS:                           1,
		RecordingStartTimeUnixMs:           0,
		SentCaptureStartedActionTimeUnixMs: 0,
	}

	for _, lines := range perFrameLines {
		ocrData.FramesMetaData = append(ocrData.FramesMetaData, frameData{
			CornerPoints: [][]point{},
			Lines:        lines,
		})
	}

	return ocrData
}

func generateTimestamps(testStartTime time.Time, perKeystrokeDelays []time.Duration) []time.Time {
	var timestamps []time.Time
	for _, delay := range perKeystrokeDelays {
		timestamps = append(timestamps, testStartTime.Add(delay))
	}

	return timestamps
}

func TestLag(t *testing.T) {
	testStartTime := time.Unix(0, 0)

	// No lag if OCR didn't recognize any lines
	AssertEqualLag(t, []time.Duration{},
		calculateLag(context.Background(), generateHostData([][]string{
			{},
			{},
			{},
		}), testStartTime, generateTimestamps(testStartTime,
			[]time.Duration{time.Second})))

	// Simple tests for a single match.
	AssertEqualLag(t, []time.Duration{0},
		calculateLag(context.Background(), generateHostData([][]string{
			{},
			{"m"},
		}), testStartTime, generateTimestamps(testStartTime,
			[]time.Duration{time.Second})))

	AssertEqualLag(t, []time.Duration{0},
		calculateLag(context.Background(), generateHostData([][]string{
			{"m"},
		}), testStartTime, generateTimestamps(testStartTime,
			[]time.Duration{0})))

	AssertEqualLag(t, []time.Duration{time.Second},
		calculateLag(context.Background(), generateHostData([][]string{
			{},
			{"m"},
		}), testStartTime, generateTimestamps(testStartTime,
			[]time.Duration{0})))

	// Longer string won't match.
	AssertEqualLag(t, []time.Duration{},
		calculateLag(context.Background(), generateHostData([][]string{
			{"mm"},
		}), testStartTime, generateTimestamps(testStartTime,
			[]time.Duration{0})))

	// This test more closely resembles the data from real usage.
	AssertEqualLag(t, []time.Duration{2 * time.Second, 3 * time.Second, 5 * time.Second},
		calculateLag(context.Background(), generateHostData([][]string{
			{"foo"},
			{"foo"},
			{"foo", "m"}, // 2 - 0 = 2
			{"foo", "m"},
			{"foo", "mm"}, // 4 - 1 = 3
			{"foo", "mm"},
			{"foo", "mm"},
			{"foo", "mmm"}, // 7 - 2 = 5
			{"foo", "mmm"},
			{"foo", "mmm"},
			{"foo", "mmm"},
		}), testStartTime, generateTimestamps(testStartTime,
			[]time.Duration{0, 1 * time.Second, 2 * time.Second})))

	// Ensure we look for matches in order.
	AssertEqualLag(t, []time.Duration{2 * time.Second, 2 * time.Second, 2 * time.Second},
		calculateLag(context.Background(), generateHostData([][]string{
			{"mmm"},
			{"mm"},
			// data before this line should be ignored.
			{"m"},
			{"mm"},
			{"mmm"},
		}), testStartTime, generateTimestamps(testStartTime,
			[]time.Duration{0, 1 * time.Second, 2 * time.Second})))
}

func TestTimeSkew(t *testing.T) {
	ocrData := generateHostData([][]string{
		{"m"},
	})

	testStartTime := time.Unix(0, 0)
	timestamps := generateTimestamps(testStartTime,
		[]time.Duration{0})
	AssertEqualLag(t, []time.Duration{0},
		calculateLag(context.Background(), ocrData, testStartTime, timestamps))

	// Move testStartTime +1 second.
	// This doesn't affect the results.
	testStartTime = time.Unix(10, 0)
	timestamps = generateTimestamps(testStartTime,
		[]time.Duration{0})
	AssertEqualLag(t, []time.Duration{0},
		calculateLag(context.Background(), ocrData, testStartTime, timestamps))

	// Update RecordingStartTimeUnixMs +1 second.
	// This adds +1 second to the lag output.
	ocrData.RecordingStartTimeUnixMs += 1000
	AssertEqualLag(t, []time.Duration{1 * time.Second},
		calculateLag(context.Background(), ocrData, testStartTime, timestamps))

	// Update SentCaptureStartedActionTimeUnixMs +1 second.
	// This removes 1 second from the lag output.
	ocrData.SentCaptureStartedActionTimeUnixMs += 1000
	AssertEqualLag(t, []time.Duration{0},
		calculateLag(context.Background(), ocrData, testStartTime, timestamps))
}
