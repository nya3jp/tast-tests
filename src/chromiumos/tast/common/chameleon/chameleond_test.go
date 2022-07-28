// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleon

import "testing"

func TestMapToAudioDataFormat(t *testing.T) {
	for _, tc := range []struct {
		input     map[string]interface{}
		expected  *AudioDataFormat
		expectErr bool
	}{
		{
			input:     map[string]interface{}{},
			expected:  &AudioDataFormat{},
			expectErr: false,
		},
		{
			input: map[string]interface{}{
				"file_type": AudioFileTypeWav.String(),
			},
			expected: &AudioDataFormat{
				FileType: AudioFileTypeWav,
			},
			expectErr: false,
		},
		{
			input: map[string]interface{}{
				"file_type":     AudioFileTypeWav.String(),
				"sample_format": AudioSampleFormatS8.String(),
				"channel":       1,
				"rate":          2,
			},
			expected: &AudioDataFormat{
				FileType:     AudioFileTypeWav,
				SampleFormat: AudioSampleFormatS8,
				Channel:      1,
				Rate:         2,
			},
			expectErr: false,
		},
		{
			input: map[string]interface{}{
				"file_type":     "a",
				"sample_format": "b",
			},
			expected: &AudioDataFormat{
				FileType:     "a",
				SampleFormat: "b",
			},
			expectErr: false,
		},
		{
			input: map[string]interface{}{
				"file_type": 1,
			},
			expected:  nil,
			expectErr: true,
		},
	} {
		actual, err := MapToAudioDataFormat(tc.input)
		if tc.expectErr {
			if err == nil {
				t.Errorf("MapToAudioDataFormat(%v) unexpectedly succeeded", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("MapToAudioDataFormat(%v) failed: %v", tc.input, err)
			}
			matches := true
			if actual == nil || tc.expected == nil {
				matches = actual == tc.expected
			} else {
				matches = actual.SampleFormat == tc.expected.SampleFormat &&
					actual.Channel == tc.expected.Channel &&
					actual.Rate == tc.expected.Rate &&
					actual.FileType == tc.expected.FileType
			}
			if !matches {
				t.Errorf("MapToAudioDataFormat(%v) = %v; want %v", tc.input, actual, tc.expected)
			}
		}
	}
}

func TestAudioDataFormatToMap(t *testing.T) {
	for _, tc := range []struct {
		input    *AudioDataFormat
		expected map[string]interface{}
	}{
		{
			input: &AudioDataFormat{},
			expected: map[string]interface{}{
				"file_type":     "",
				"sample_format": "",
				"channel":       0,
				"rate":          0,
			},
		},
		{
			input: &AudioDataFormat{
				FileType:     AudioFileTypeWav,
				SampleFormat: AudioSampleFormatS8,
				Channel:      1,
				Rate:         2,
			},
			expected: map[string]interface{}{
				"file_type":     AudioFileTypeWav.String(),
				"sample_format": AudioSampleFormatS8.String(),
				"channel":       1,
				"rate":          2,
			},
		},
	} {
		actual := tc.input.Map()
		matches := true
		for key := range tc.expected {
			if tc.expected[key] != actual[key] {
				matches = false
			}
		}
		if !matches {
			t.Errorf("AudioDataFormat{%v}.Map() = %v; want %v", tc.input, actual, tc.expected)
		}
	}
}

func TestMapToVideoParams(t *testing.T) {
	for _, tc := range []struct {
		input     map[string]interface{}
		expected  *VideoParams
		expectErr bool
	}{
		{
			input:     map[string]interface{}{},
			expected:  &VideoParams{},
			expectErr: false,
		},
		{
			input: map[string]interface{}{
				"clock": "unexpected string",
			},
			expected:  nil,
			expectErr: true,
		},
		{
			input: map[string]interface{}{
				"clock":          1.23,
				"htotal":         1,
				"hactive":        2,
				"hsync_width":    3,
				"hsync_offset":   4,
				"hsync_polarity": 5,
				"vtotal":         6,
				"vactive":        7,
				"vsync_width":    8,
				"vsync_offset":   9,
				"vsync_polarity": 10,
				"bpc":            11,
				"interlaced":     true,
			},
			expected: &VideoParams{
				Clock:         1.23,
				HTotal:        1,
				HActive:       2,
				HSyncWidth:    3,
				HSyncOffset:   4,
				HSyncPolarity: 5,
				VTotal:        6,
				VActive:       7,
				VSyncWidth:    8,
				VSyncOffset:   9,
				VSyncPolarity: 10,
				BPC:           11,
				Interlaced:    true,
			},
			expectErr: false,
		},
	} {
		actual, err := MapToVideoParams(tc.input)
		if tc.expectErr {
			if err == nil {
				t.Errorf("MapToVideoParams(%v) unexpectedly succeeded", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("MapToVideoParams(%v) failed: %v", tc.input, err)
			}
			matches := true
			if actual == nil || tc.expected == nil {
				matches = actual == tc.expected
			} else {
				matches = actual.Clock == tc.expected.Clock &&
					actual.HTotal == tc.expected.HTotal &&
					actual.HActive == tc.expected.HActive &&
					actual.HSyncWidth == tc.expected.HSyncWidth &&
					actual.HSyncOffset == tc.expected.HSyncOffset &&
					actual.HSyncPolarity == tc.expected.HSyncPolarity &&
					actual.VTotal == tc.expected.VTotal &&
					actual.VActive == tc.expected.VActive &&
					actual.VSyncWidth == tc.expected.VSyncWidth &&
					actual.VSyncOffset == tc.expected.VSyncOffset &&
					actual.VSyncPolarity == tc.expected.VSyncPolarity &&
					actual.BPC == tc.expected.BPC &&
					actual.Interlaced == tc.expected.Interlaced
			}
			if !matches {
				t.Errorf("MapToAudioDataFormat(%q) = %v; want %v", tc.input, actual, tc.expected)
			}
		}
	}
}
