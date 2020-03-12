// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mtbferrors

import (
	"fmt"

	"chromiumos/tast/errors"
)

// MTBFError wraps tast errors package for stack trace
// reference https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#error-construction
type MTBFError struct {
	*errors.E
	errCode MTBFErrCode
}

// New creates a new MTBFError with error code and error cause
func New(errCode MTBFErrCode, cause error, args ...interface{}) *MTBFError {
	code := errCode.code
	errFmt := errCode.format

	args = append([]interface{}{code}, args...)
	mtbfErrStr := fmt.Sprintf("[ERR-%d] "+errFmt, args...)

	return &MTBFError{
		errCode: errCode,
		E:       errors.Wrap(cause, mtbfErrStr),
	}
}

// ErrorCode returns error code
func (e MTBFError) ErrorCode() int {
	return e.errCode.code
}
