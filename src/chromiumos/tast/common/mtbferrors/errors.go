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
	errType MTBFErrType
}

// New creates a new MTBFError with error code and error cause
func New(code MTBFErrCode, cause error, args ...interface{}) *MTBFError {
	errCode := MTBFErrCode(int(code) % 1000)
	errType := MTBFErrType(int(code) / 1000)

	args = append([]interface{}{code}, args...)
	mtbfErrStr := fmt.Sprintf("[ERR-%d] "+errCodeToString[code], args...)

	return &MTBFError{
		errCode: errCode,
		errType: errType,
		E:       errors.Wrap(cause, mtbfErrStr),
	}
}

// Implements golang built-in Error interface method
func (e MTBFError) Error() string {
	return e.E.Error()
}

// ErrorCode returns error code
func (e MTBFError) ErrorCode() MTBFErrCode {
	return e.errCode
}

// ErrorType returns error type
func (e MTBFError) ErrorType() MTBFErrType {
	return e.errType
}
