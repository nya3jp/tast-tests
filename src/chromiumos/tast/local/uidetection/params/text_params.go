// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package params

// TextParams provides all of the ways which you can configure a word detection
// or a textblock detection.
type TextParams struct {
	// RegexMode indicates whether the query string is a regex or not.
	RegexMode bool
	// DisableApproxMatch disables the approximate match.
	// approxmiate match is enabled by default to tolerate recognition errors,
	// so that some similar characters (i.e., "5" and "s") are treated the same.
	// Normally, you don't want to turn the approxmiate match off.
	DisableApproxMatch bool
	// MaxEditDistance is the Levenshtein distance threshold.
	// For example "string" and "sting" is the same match if MaxEditDistance=1.
	// NOTE: this param is applicable only if RegexMode is False.
	MaxEditDistance int32
}

// DefaultTextParams return params with default values.
func DefaultTextParams() *TextParams {
	return &TextParams{
		MaxEditDistance: 1,
	}
}

// TextParam is a modifier to apply to Params.
type TextParam = func(*TextParams)

// RegexMode controls the RegexMode param.
func RegexMode(regexMode bool) TextParam {
	return func(o *TextParams) { o.RegexMode = regexMode }
}

// DisableApproxMatch controls the DisableApproxMatch param.
func DisableApproxMatch(disableApproxMatch bool) TextParam {
	return func(o *TextParams) { o.DisableApproxMatch = disableApproxMatch }
}

// MaxEditDistance controls the MaxEditDistance param.
func MaxEditDistance(maxEditDistance int32) TextParam {
	return func(o *TextParams) { o.MaxEditDistance = maxEditDistance }
}
