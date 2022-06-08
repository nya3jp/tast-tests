// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	pbchameleond "go.chromium.org/chromiumos/config/go/platform/chameleon/chameleond/rpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/chameleond"
	"chromiumos/tast/testing"
)

type checkChameleondServiceMethodsTestCase struct {
	callMethod             func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error)
	expectedResponse       interface{}
	expectExactReturnMatch bool
	checkResponse          func(ctx context.Context, response interface{}) error
}

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckChameleondServiceMethods,
		Desc: "Calls every available gRPC method in the ChameleondService and ensures that an non-error response is returned",
		Contacts: []string{
			"jaredbennett@google.com",
		},
		Attr:         []string{},
		ServiceDeps:  []string{},
		SoftwareDeps: []string{},
		Fixture:      "simpleChameleond",
		Timeout:      1 * time.Minute,
		Params: []testing.Param{
			{
				Name: "reset",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.Reset(ctx, &pbchameleond.ResetRequest{})
					},
					expectedResponse: &pbchameleond.ResetResponse{},
				},
			},
			{
				Name: "get_detected_status",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetDetectedStatus(ctx, &pbchameleond.GetDetectedStatusRequest{})
					},
					expectedResponse: &pbchameleond.GetDetectedStatusResponse{},
					checkResponse: func(ctx context.Context, response interface{}) error {
						r := response.(*pbchameleond.GetDetectedStatusResponse)
						if r.DetectedStatuses == nil || len(r.DetectedStatuses) == 0 {
							return errors.New("expected at least one detected status")
						}
						atLeastOneNonDefaultDeviceID := false
						atLeastOneNonDefaultStatus := false
						for _, status := range r.DetectedStatuses {
							if status.DeviceId != 0 {
								atLeastOneNonDefaultDeviceID = true
							}
							if status.Status {
								atLeastOneNonDefaultStatus = true
							}
						}
						if !atLeastOneNonDefaultDeviceID {
							return errors.New("no non-default device id")
						}
						if !atLeastOneNonDefaultStatus {
							return errors.New("no non-default status")
						}
						return nil
					},
				},
			},
			{
				Name: "has_device",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.HasDevice(ctx, &pbchameleond.HasDeviceRequest{
							DeviceId: pbchameleond.PortId_BLUETOOTH_AUDIO,
						})
					},
					expectedResponse: &pbchameleond.HasDeviceResponse{},
				},
			},
			{
				Name: "get_supported_ports",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetSupportedPorts(ctx, &pbchameleond.GetSupportedPortsRequest{})
					},
					expectedResponse: &pbchameleond.GetSupportedPortsResponse{},
					checkResponse: func(ctx context.Context, response interface{}) error {
						r := response.(*pbchameleond.GetSupportedPortsResponse)
						if r.SupportedPorts == nil || len(r.SupportedPorts) == 0 {
							return errors.New("expected at least one supported port")
						}
						return nil
					},
				},
			},
			{
				Name: "get_supported_inputs",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetSupportedInputs(ctx, &pbchameleond.GetSupportedInputsRequest{})
					},
					expectedResponse: &pbchameleond.GetSupportedInputsResponse{},
				},
			},
			{
				Name: "get_supported_outputs",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetSupportedOutputs(ctx, &pbchameleond.GetSupportedOutputsRequest{})
					},
					expectedResponse: &pbchameleond.GetSupportedOutputsResponse{},
				},
			},
			{
				Name: "is_physical_plugged",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.IsPhysicalPlugged(ctx, &pbchameleond.IsPhysicalPluggedRequest{
							PortId: pbchameleond.PortId_BLUETOOTH_HID_GAMEPAD,
						})
					},
					expectedResponse: &pbchameleond.IsPhysicalPluggedResponse{},
				},
			},
			{
				Name: "probe_ports",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.ProbePorts(ctx, &pbchameleond.ProbePortsRequest{})
					},
					expectedResponse: &pbchameleond.ProbePortsResponse{},
					checkResponse: func(ctx context.Context, response interface{}) error {
						r := response.(*pbchameleond.ProbePortsResponse)
						if r.PortsConnectedToDut == nil || len(r.PortsConnectedToDut) == 0 {
							return errors.New("expected at least one port to be connected to dut")
						}
						return nil
					},
				},
			},
			{
				Name: "probe_inputs",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.ProbeInputs(ctx, &pbchameleond.ProbeInputsRequest{})
					},
					expectedResponse: &pbchameleond.ProbeInputsResponse{},
				},
			},
			{
				Name: "probe_outputs",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.ProbeOutputs(ctx, &pbchameleond.ProbeOutputsRequest{})
					},
					expectedResponse: &pbchameleond.ProbeOutputsResponse{},
				},
			},
			{
				// TODO this always returns null for some reason, perhaps a chameleond problem
				Name: "get_connector_type",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetConnectorType(ctx, &pbchameleond.GetConnectorTypeRequest{
							PortId: pbchameleond.PortId_HDMI,
						})
					},
					expectedResponse: &pbchameleond.GetConnectorTypeResponse{
						ConnectorType: "HDMI",
					},
					expectExactReturnMatch: true,
				},
			},
			{
				Name: "is_plugged",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.IsPlugged(ctx, &pbchameleond.IsPluggedRequest{
							PortId: pbchameleond.PortId_BLUETOOTH_HID_GAMEPAD,
						})
					},
					expectedResponse: &pbchameleond.IsPluggedResponse{},
				},
			},
			{
				Name: "plug",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.Plug(ctx, &pbchameleond.PlugRequest{
							PortId: pbchameleond.PortId_BLUETOOTH_HID_GAMEPAD,
						})
					},
					expectedResponse: &pbchameleond.PlugResponse{},
				},
			},
			{
				Name: "unplug",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.Unplug(ctx, &pbchameleond.UnplugRequest{
							PortId: pbchameleond.PortId_BLUETOOTH_HID_GAMEPAD,
						})
					},
					expectedResponse: &pbchameleond.UnplugResponse{},
				},
			},
			{
				Name: "get_mac_address",
				Val: &checkChameleondServiceMethodsTestCase{
					callMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetMacAddress(ctx, &pbchameleond.GetMacAddressRequest{})
					},
					expectedResponse: &pbchameleond.GetMacAddressResponse{},
				},
			},
		},
	})
}

// CheckChameleondServiceMethods tests a ChameleondService RPC call as
// defined in the checkChameleondServiceMethodsTestCase.
func CheckChameleondServiceMethods(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*chameleond.TestFixture)
	client := tf.CMS.ChameleondService

	tc := s.Param().(*checkChameleondServiceMethodsTestCase)

	response, err := tc.callMethod(ctx, client)
	if err != nil {
		s.Fatal("Method call returned error: ", err)
	}
	if response == nil {
		s.Fatal("Method call returned nil response")
	}
	if tc.expectedResponse != nil {
		responseType := reflect.TypeOf(response)
		expectedResponseType := reflect.TypeOf(tc.expectedResponse)
		if responseType != expectedResponseType {
			s.Fatalf("Method call response type was expected to be %q, but got %q", expectedResponseType, responseType)
		}
		if tc.expectExactReturnMatch && !reflect.DeepEqual(tc.expectedResponse, response) {
			responseJSON, err := json.Marshal(response)
			if err != nil {
				s.Fatal("Method call response differs from expected exact match, and failed to marshal response to JSON: ", err)
			}
			expectedResponseJSON, err := json.Marshal(tc.expectedResponse)
			if err != nil {
				s.Fatal("Method call response differs from expected exact match, and failed to marshal expected response to JSON: ", err)
			}
			s.Fatalf("Method call response differs from expected exact match: expected %s got %s", string(expectedResponseJSON), string(responseJSON))
		}
	}
	if tc.checkResponse != nil {
		if err := tc.checkResponse(ctx, response); err != nil {
			responseJSON, _ := json.Marshal(response)
			if err != nil {
				s.Fatal("Method call response response failed check: ", err)
			}
			s.Fatalf("Method call response failed check. Response as JSON: %s. Check error: %v", responseJSON, err)
		}
	}
}
