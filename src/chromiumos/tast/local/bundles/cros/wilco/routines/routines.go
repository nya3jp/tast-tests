// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package routines

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

// CallRunRoutine executes a RunRoutine call and does sanity checks on the
// result.
func CallRunRoutine(ctx context.Context, request dtcpb.RunRoutineRequest, response *dtcpb.RunRoutineResponse) error {
	if err := wilco.DPSLSendMessage(ctx, "RunRoutine", &request, response); err != nil {
		return errors.Wrapf(err, "unable to run routine %s", request.Routine)
	}

	switch response.Status {
	case dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED_TO_START,
		dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLING,
		dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_REMOVED:

		return errors.Errorf("unpexected status: %s", response.Status)
	}

	return nil
}

// CallGetRoutineUpdate executes a GetRoutineUpdate call.
func CallGetRoutineUpdate(ctx context.Context,
	request dtcpb.GetRoutineUpdateRequest, response *dtcpb.GetRoutineUpdateResponse) error {

	if err := wilco.DPSLSendMessage(ctx, "GetRoutineUpdate", &request, response); err != nil {
		return errors.Wrap(err, "unable to get update on routine")
	}

	return nil
}

// GetRoutineStatus gets the status of a routine by calling GET_STATUS.
func GetRoutineStatus(ctx context.Context, uuid int32, includeOutput bool,
	response *dtcpb.GetRoutineUpdateResponse) error {

	request := dtcpb.GetRoutineUpdateRequest{
		Uuid:          uuid,
		Command:       dtcpb.GetRoutineUpdateRequest_GET_STATUS,
		IncludeOutput: includeOutput,
	}

	return CallGetRoutineUpdate(ctx, request, response)
}

// CancelRoutine cancels a routine by calling CANCEL.
func CancelRoutine(ctx context.Context, uuid int32) error {
	request := dtcpb.GetRoutineUpdateRequest{
		Uuid:          uuid,
		Command:       dtcpb.GetRoutineUpdateRequest_CANCEL,
		IncludeOutput: false,
	}

	response := dtcpb.GetRoutineUpdateResponse{}
	return CallGetRoutineUpdate(ctx, request, &response)
}

// ResumeRoutine resumes a routine by calling RESUME.
func ResumeRoutine(ctx context.Context, uuid int32) error {
	request := dtcpb.GetRoutineUpdateRequest{
		Uuid:          uuid,
		Command:       dtcpb.GetRoutineUpdateRequest_RESUME,
		IncludeOutput: false,
	}

	response := dtcpb.GetRoutineUpdateResponse{}
	return CallGetRoutineUpdate(ctx, request, &response)
}

// RemoveRoutine removes a routine by calling REMOVE.
func RemoveRoutine(ctx context.Context, uuid int32) error {
	request := dtcpb.GetRoutineUpdateRequest{
		Uuid:          uuid,
		Command:       dtcpb.GetRoutineUpdateRequest_REMOVE,
		IncludeOutput: false,
	}

	response := dtcpb.GetRoutineUpdateResponse{}
	return CallGetRoutineUpdate(ctx, request, &response)
}

// WaitUntilRoutineChangesState polls until the status of a routine changes from
// the state parameter or times out.
func WaitUntilRoutineChangesState(ctx context.Context, uuid int32,
	state dtcpb.DiagnosticRoutineStatus, timeout time.Duration) error {

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		response := dtcpb.GetRoutineUpdateResponse{}
		err := GetRoutineStatus(ctx, uuid, true, &response)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "unable to get routine status"))
		}

		if response.Status == state {
			return errors.Errorf("state is still %s", response.Status)
		}

		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "routine failed to change state")
	}

	return nil
}
