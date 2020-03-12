// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mtbferrors

import (
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// MTBFError wraps tast errors package for stack trace.
// reference https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#error-construction
type MTBFError struct {
	*errors.E
	errCode MTBFErrCode
}

// New creates a new MTBFError with error code and error cause.
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

// ErrorCode returns error code.
func (e MTBFError) ErrorCode() int {
	return e.errCode.code
}

// NewCatsNodeErr creates a new MTBFError with error code and error cause.
func NewCatsNodeErr(errCode CatsErrCode, cause error, args ...interface{}) *MTBFError {

	code := errCode.code
	errFmt := errCode.format
	taskID := errCode.TaskID
	rptURL := errCode.TaskRptURL

	if len(errCode.CatsNodeOrigCode) != 0 {
		origCode, err := strconv.Atoi(errCode.CatsNodeOrigCode)
		if err != nil {
			code = CatsErr6013.code
			errFmt = CatsErr6013.format
		} else {
			code = origCode
		}
	}

	newArgs := []interface{}{code}
	if len(taskID) > 0 {
		newArgs = append(newArgs, taskID)
	} else {
		newArgs = append(newArgs, "null")
	}
	if code == CatsErr6013.code {
		newArgs = append(newArgs, errCode.CatsNodeOrigCode)
	}
	newArgs = append(newArgs, args)

	mtbfErrStr := fmt.Sprintf("[ERR-%d] [ID-%s] CATS Case fail by: "+errFmt, newArgs...)
	if len(rptURL) > 0 {
		mtbfErrStr = mtbfErrStr + fmt.Sprintf(" [RptURL-%s]", rptURL)
	}

	newErrCode := &MTBFErrCode{
		code:   code,
		format: errFmt,
	}
	return &MTBFError{
		errCode: *newErrCode,
		E:       errors.Wrap(cause, mtbfErrStr),
	}

}

// NewGRPCErr re-wrap the GRPC error.
func NewGRPCErr(err error) error {
	if strings.Contains(err.Error(), "rpc error: code = Unavailable desc = transport is closing") {
		return New(GRPCTransportClosing, err)
	}
	return err
}
