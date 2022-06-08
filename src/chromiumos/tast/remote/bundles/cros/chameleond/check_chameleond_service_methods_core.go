// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"context"
	"time"

	pbchameleond "go.chromium.org/chromiumos/config/go/platform/chameleon/chameleond/rpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/chameleond/util"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckChameleondServiceMethodsCore,
		Desc: "Calls every available core gRPC endpoint in the ChameleondService as defined in the test cases",
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
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.Reset(ctx, &pbchameleond.ResetRequest{})
					},
					ExpectedResponse: &pbchameleond.ResetResponse{},
				},
			},
			{
				Name: "get_detected_status",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetDetectedStatus(ctx, &pbchameleond.GetDetectedStatusRequest{})
					},
					ExpectedResponse: &pbchameleond.GetDetectedStatusResponse{},
					CheckResponse: func(ctx context.Context, response interface{}) error {
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
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.HasDevice(ctx, &pbchameleond.HasDeviceRequest{
							DeviceId: pbchameleond.PortId_BLUETOOTH_AUDIO,
						})
					},
					ExpectedResponse: &pbchameleond.HasDeviceResponse{},
				},
			},
			{
				Name: "get_supported_ports",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetSupportedPorts(ctx, &pbchameleond.GetSupportedPortsRequest{})
					},
					ExpectedResponse: &pbchameleond.GetSupportedPortsResponse{},
					CheckResponse: func(ctx context.Context, response interface{}) error {
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
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetSupportedInputs(ctx, &pbchameleond.GetSupportedInputsRequest{})
					},
					ExpectedResponse: &pbchameleond.GetSupportedInputsResponse{},
				},
			},
			{
				Name: "get_supported_outputs",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetSupportedOutputs(ctx, &pbchameleond.GetSupportedOutputsRequest{})
					},
					ExpectedResponse: &pbchameleond.GetSupportedOutputsResponse{},
				},
			},
			{
				Name: "is_physical_plugged",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.IsPhysicalPlugged(ctx, &pbchameleond.IsPhysicalPluggedRequest{
							PortId: pbchameleond.PortId_BLUETOOTH_HID_GAMEPAD,
						})
					},
					ExpectedResponse: &pbchameleond.IsPhysicalPluggedResponse{},
				},
			},
			{
				Name: "probe_ports",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.ProbePorts(ctx, &pbchameleond.ProbePortsRequest{})
					},
					ExpectedResponse: &pbchameleond.ProbePortsResponse{},
					CheckResponse: func(ctx context.Context, response interface{}) error {
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
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.ProbeInputs(ctx, &pbchameleond.ProbeInputsRequest{})
					},
					ExpectedResponse: &pbchameleond.ProbeInputsResponse{},
				},
			},
			{
				Name: "probe_outputs",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.ProbeOutputs(ctx, &pbchameleond.ProbeOutputsRequest{})
					},
					ExpectedResponse: &pbchameleond.ProbeOutputsResponse{},
				},
			},
			{
				// TODO this always returns null for some reason, perhaps a chameleond problem
				Name: "get_connector_type",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetConnectorType(ctx, &pbchameleond.GetConnectorTypeRequest{
							PortId: pbchameleond.PortId_HDMI,
						})
					},
					ExpectedResponse: &pbchameleond.GetConnectorTypeResponse{
						ConnectorType: "HDMI",
					},
					ExpectExactReturnMatch: true,
				},
			},
			{
				Name: "is_plugged",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.IsPlugged(ctx, &pbchameleond.IsPluggedRequest{
							PortId: pbchameleond.PortId_BLUETOOTH_HID_GAMEPAD,
						})
					},
					ExpectedResponse: &pbchameleond.IsPluggedResponse{},
				},
			},
			{
				Name: "plug",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.Plug(ctx, &pbchameleond.PlugRequest{
							PortId: pbchameleond.PortId_BLUETOOTH_HID_GAMEPAD,
						})
					},
					ExpectedResponse: &pbchameleond.PlugResponse{},
				},
			},
			{
				Name: "unplug",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.Unplug(ctx, &pbchameleond.UnplugRequest{
							PortId: pbchameleond.PortId_BLUETOOTH_HID_GAMEPAD,
						})
					},
					ExpectedResponse: &pbchameleond.UnplugResponse{},
				},
			},
			{
				Name: "get_mac_address",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetMacAddress(ctx, &pbchameleond.GetMacAddressRequest{})
					},
					ExpectedResponse: &pbchameleond.GetMacAddressResponse{},
				},
			},
		},
	})
}

// CheckChameleondServiceMethodsCore tests every core ChameleondService RPC
// endpoint as defined in the test cases.
func CheckChameleondServiceMethodsCore(ctx context.Context, s *testing.State) {
	util.CheckChameleondServiceMethods(ctx, s)
}
