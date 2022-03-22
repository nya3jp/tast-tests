// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

// CustomIconParams provides all of the ways which you can configure
// a custom icon detection.
//
// NOTE: The default values are the recommended values. The users are not
// expected to modify them if the test is passing with the defaults.
// If the defaults are not working, the users will need to find a proper value
// with the trial and error method.
//
// See https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/using_uidetection.md
// for more details.
type CustomIconParams struct {
	// MatchCount is the limit of number of matches.
	// Set to -1 to not limit the number of matches.
	MatchCount int32
	// MinConfidence is the threshold in the range [0.0, 1.0] below which
	// the matches will be considered as non-existent.
	MinConfidence float64
}

// DefaultCustomIconParams return params with default values.
func DefaultCustomIconParams() *CustomIconParams {
	return &CustomIconParams{
		MatchCount:    1,
		MinConfidence: 0.7,
	}
}

// CustomIconParam is a modifier to apply to CustomIconParams.
type CustomIconParam = func(*CustomIconParams)

// MatchCount controls the MatchCount param.
func MatchCount(matchCount int32) CustomIconParam {
	return func(o *CustomIconParams) { o.MatchCount = matchCount }
}

// MinConfidence controls the min confidence threshold.
func MinConfidence(minConfidence float64) CustomIconParam {
	return func(o *CustomIconParams) { o.MinConfidence = minConfidence }
}
