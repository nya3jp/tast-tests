// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"encoding/json"
	"reflect"

	pbchameleond "go.chromium.org/chromiumos/config/go/platform/chameleon/chameleond/rpc"

	"chromiumos/tast/remote/chameleond"
	"chromiumos/tast/testing"
)

// CheckChameleondServiceMethodsTestCase describes a test case for checking a
// ChameleondService gRPC method. This is expected to be used with
// CheckChameleondServiceMethods.
type CheckChameleondServiceMethodsTestCase struct {
	CallMethod             func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error)
	ExpectedResponse       interface{}
	ExpectExactReturnMatch bool
	CheckResponse          func(ctx context.Context, response interface{}) error
}

// CheckChameleondServiceMethods tests a ChameleondService RPC call as
// defined in the CheckChameleondServiceMethodsTestCase.
func CheckChameleondServiceMethods(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*chameleond.TestFixture)
	client := tf.CMS.ChameleondService

	tc := s.Param().(*CheckChameleondServiceMethodsTestCase)

	response, err := tc.CallMethod(ctx, client)
	if err != nil {
		s.Fatal("Method call returned error: ", err)
	}
	if response == nil {
		s.Fatal("Method call returned nil response")
	}
	if tc.ExpectedResponse != nil {
		responseType := reflect.TypeOf(response)
		expectedResponseType := reflect.TypeOf(tc.ExpectedResponse)
		if responseType != expectedResponseType {
			s.Fatalf("Method call response type was expected to be %q, but got %q", expectedResponseType, responseType)
		}
		if tc.ExpectExactReturnMatch && !reflect.DeepEqual(tc.ExpectedResponse, response) {
			responseJSON, err := json.Marshal(response)
			if err != nil {
				s.Fatal("Method call response differs from expected exact match, and failed to marshal response to JSON: ", err)
			}
			expectedResponseJSON, err := json.Marshal(tc.ExpectedResponse)
			if err != nil {
				s.Fatal("Method call response differs from expected exact match, and failed to marshal expected response to JSON: ", err)
			}
			s.Fatalf("Method call response differs from expected exact match: expected %s got %s", string(expectedResponseJSON), string(responseJSON))
		}
	}
	if tc.CheckResponse != nil {
		if err := tc.CheckResponse(ctx, response); err != nil {
			responseJSON, _ := json.Marshal(response)
			if err != nil {
				s.Fatal("Method call response response failed check: ", err)
			}
			s.Fatalf("Method call response failed check. Response as JSON: %s. Check error: %v", responseJSON, err)
		}
	}
}
