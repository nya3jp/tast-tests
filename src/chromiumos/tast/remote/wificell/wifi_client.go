// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// WifiClient is a wrapper of ShillServiceClient to simplify gRPC calls
// e.g. handle complex streaming gRPCs, hide gRPC request/response, etc.
// Users can still access the raw gRPC with WifiClient.ShillServiceClient.
type WifiClient struct {
	wifi.ShillServiceClient
}

// DiscoverBSSID discovers a service with the given properties.
func (cli *WifiClient) DiscoverBSSID(ctx context.Context, bssid, iface string, ssid []byte) error {
	ctx, st := timing.Start(ctx, "DiscoverBSSID")
	defer st.End()
	request := &wifi.DiscoverBSSIDRequest{
		Bssid:     bssid,
		Interface: iface,
		Ssid:      ssid,
	}
	if _, err := cli.ShillServiceClient.DiscoverBSSID(ctx, request); err != nil {
		return err
	}

	return nil
}

// RequestRoam requests DUT to roam to the specified BSSID and waits until the DUT has roamed.
func (cli *WifiClient) RequestRoam(ctx context.Context, iface, bssid string, timeout time.Duration) error {
	request := &wifi.RequestRoamRequest{
		InterfaceName: iface,
		Bssid:         bssid,
		Timeout:       timeout.Nanoseconds(),
	}
	if _, err := cli.ShillServiceClient.RequestRoam(ctx, request); err != nil {
		return err
	}

	return nil
}

// Reassociate triggers reassociation with the current AP and waits until it has reconnected or the timeout expires.
func (cli *WifiClient) Reassociate(ctx context.Context, iface string, timeout time.Duration) error {
	_, err := cli.ShillServiceClient.Reassociate(ctx, &wifi.ReassociateRequest{
		InterfaceName: iface,
		Timeout:       timeout.Nanoseconds(),
	})
	return err
}

// FlushBSS flushes BSS entries over the specified age from wpa_supplicant's cache.
func (cli *WifiClient) FlushBSS(ctx context.Context, iface string, age time.Duration) error {
	req := &wifi.FlushBSSRequest{
		InterfaceName: iface,
		Age:           age.Nanoseconds(),
	}
	_, err := cli.ShillServiceClient.FlushBSS(ctx, req)
	return err
}

// AssureDisconnect assures that the WiFi service has disconnected within timeout.
func (cli *WifiClient) AssureDisconnect(ctx context.Context, servicePath string, timeout time.Duration) error {
	req := &wifi.AssureDisconnectRequest{
		ServicePath: servicePath,
		Timeout:     timeout.Nanoseconds(),
	}
	if _, err := cli.ShillServiceClient.AssureDisconnect(ctx, req); err != nil {
		return err
	}
	return nil
}

// QueryService queries shill information of selected service.
func (cli *WifiClient) QueryService(ctx context.Context) (*wifi.QueryServiceResponse, error) {
	selectedSvcResp, err := cli.ShillServiceClient.SelectedService(ctx, &empty.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get selected service")
	}

	req := &wifi.QueryServiceRequest{
		Path: selectedSvcResp.ServicePath,
	}
	resp, err := cli.ShillServiceClient.QueryService(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the service information")
	}

	return resp, nil
}

// Interface returns the WiFi interface name of the DUT.
func (cli *WifiClient) Interface(ctx context.Context) (string, error) {
	netIf, err := cli.ShillServiceClient.GetInterface(ctx, &empty.Empty{})
	if err != nil {
		return "", errors.Wrap(err, "failed to get the WiFi interface name")
	}
	return netIf.Name, nil
}

// CurrentTime returns the current time on DUT.
func (cli *WifiClient) CurrentTime(ctx context.Context) (time.Time, error) {
	res, err := cli.ShillServiceClient.GetCurrentTime(ctx, &empty.Empty{})
	if err != nil {
		return time.Time{}, errors.Wrap(err, "failed to get the current DUT time")
	}
	currentTime := time.Unix(res.NowSecond, res.NowNanosecond)
	return currentTime, nil
}

// ShillProperty holds a shill service property with it's expected and unexpected values.
type ShillProperty struct {
	Property         string
	ExpectedValues   []interface{}
	UnexpectedValues []interface{}
	Method           wifi.ExpectShillPropertyRequest_CheckMethod
}

// ExpectShillProperty is a wrapper for the streaming gRPC call ExpectShillProperty.
// It takes an array of ShillProperty, an array of shill properties to monitor, and
// a shill service path. It returns a function that waites for the expected property
// changes and returns the monitor results.
func (cli *WifiClient) ExpectShillProperty(ctx context.Context, objectPath string, props []*ShillProperty, monitorProps []string) (func() ([]protoutil.ShillPropertyHolder, error), error) {
	var expectedProps []*wifi.ExpectShillPropertyRequest_Criterion
	for _, prop := range props {
		var anyOfVals []*wifi.ShillVal
		for _, shillState := range prop.ExpectedValues {
			state, err := protoutil.ToShillVal(shillState)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert property name to ShillVal")
			}
			anyOfVals = append(anyOfVals, state)
		}

		var noneOfVals []*wifi.ShillVal
		for _, shillState := range prop.UnexpectedValues {
			state, err := protoutil.ToShillVal(shillState)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert property name to ShillVal")
			}
			noneOfVals = append(noneOfVals, state)
		}

		shillPropReqCriterion := &wifi.ExpectShillPropertyRequest_Criterion{
			Key:    prop.Property,
			AnyOf:  anyOfVals,
			NoneOf: noneOfVals,
			Method: prop.Method,
		}
		expectedProps = append(expectedProps, shillPropReqCriterion)
	}

	req := &wifi.ExpectShillPropertyRequest{
		ObjectPath:   objectPath,
		Props:        expectedProps,
		MonitorProps: monitorProps,
	}

	stream, err := cli.ShillServiceClient.ExpectShillProperty(ctx, req)
	if err != nil {
		return nil, err
	}

	ready, err := stream.Recv()
	if err != nil || ready.Key != "" {
		// Error due to expecting an empty response as ready signal.
		return nil, errors.New("failed to get the ready signal")
	}

	// Get the expected properties and values.
	waitForProperties := func() ([]protoutil.ShillPropertyHolder, error) {
		for {
			resp, err := stream.Recv()
			if err != nil {
				return nil, errors.Wrap(err, "failed to get the expected properties")
			}

			if resp.MonitorDone {
				return protoutil.DecodeFromShillPropertyChangedSignalList(resp.Props)
			}

			// Now we get the matched state change in resp.
			stateVal, err := protoutil.FromShillVal(resp.Val)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert property name to ShillVal")
			}
			testing.ContextLogf(ctx, "The current WiFi service %s: %v", resp.Key, stateVal)
		}
	}

	return waitForProperties, nil
}

// EAPAuthSkipped is a wrapper for the streaming gRPC call EAPAuthSkipped.
// It returns a function that waits and verifies the EAP authentication is skipped or not in the next connection.
func (cli *WifiClient) EAPAuthSkipped(ctx context.Context) (func() (bool, error), error) {
	recv, err := cli.ShillServiceClient.EAPAuthSkipped(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}
	s, err := recv.Recv()
	if err != nil {
		return nil, errors.Wrap(err, "failed to receive ready signal from EAPAuthSkipped")
	}
	if s.Skipped {
		return nil, errors.New("unexpected ready signal: got true, want false")
	}
	return func() (bool, error) {
		resp, err := recv.Recv()
		if err != nil {
			return false, errors.Wrap(err, "failed to receive from EAPAuthSkipped")
		}
		return resp.Skipped, nil
	}, nil
}

// DisconnectReason is a wrapper for the streaming gRPC call DisconnectReason.
// It returns a function that waits for the wpa_supplicant DisconnectReason
// property change, and returns the disconnection reason code.
func (cli *WifiClient) DisconnectReason(ctx context.Context) (func() (int32, error), error) {
	recv, err := cli.ShillServiceClient.DisconnectReason(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}
	ready, err := recv.Recv()
	if err != nil || ready.Reason != 0 {
		// Error due to expecting an empty response as ready signal.
		return nil, errors.New("failed to get the ready signal")
	}
	return func() (int32, error) {
		resp, err := recv.Recv()
		if err != nil {
			return 0, errors.Wrap(err, "failed to receive from DisconnectReason")
		}
		return resp.Reason, nil
	}, nil
}

// SuspendAssertConnect suspends the DUT for wakeUpTimeout seconds through gRPC and returns the duration from resume to connect.
func (cli *WifiClient) SuspendAssertConnect(ctx context.Context, wakeUpTimeout time.Duration) (time.Duration, error) {
	service, err := cli.ShillServiceClient.SelectedService(ctx, &empty.Empty{})
	if err != nil {
		return 0, errors.Wrap(err, "failed to get selected service")
	}
	resp, err := cli.ShillServiceClient.SuspendAssertConnect(ctx, &wifi.SuspendAssertConnectRequest{
		WakeUpTimeout: wakeUpTimeout.Nanoseconds(),
		ServicePath:   service.ServicePath,
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to suspend and assert connection")
	}
	return time.Duration(resp.ReconnectTime), nil
}

// Suspend suspends the DUT for wakeUpTimeout seconds through gRPC.
// This call will fail when the DUT wake up early. If the caller expects the DUT to
// wake up early, please use the Suspend gRPC to specify the detailed options.
func (cli *WifiClient) Suspend(ctx context.Context, wakeUpTimeout time.Duration) error {
	req := &wifi.SuspendRequest{
		WakeUpTimeout:  wakeUpTimeout.Nanoseconds(),
		CheckEarlyWake: true,
	}
	_, err := cli.ShillServiceClient.Suspend(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed to suspend")
	}
	return nil
}

// DisableMACRandomize disables MAC randomization on DUT if supported, this
// is useful for tests verifying probe requests from DUT.
// On success, a shortened context and cleanup function is returned.
func (cli *WifiClient) DisableMACRandomize(ctx context.Context) (shortenCtx context.Context, cleanupFunc func() error, retErr error) {
	// If MAC randomization setting is supported, disable MAC randomization
	// as we're filtering the packets with MAC address.
	if supResp, err := cli.ShillServiceClient.MACRandomizeSupport(ctx, &empty.Empty{}); err != nil {
		return ctx, nil, errors.Wrap(err, "failed to get if MAC randomization is supported")
	} else if supResp.Supported {
		resp, err := cli.ShillServiceClient.GetMACRandomize(ctx, &empty.Empty{})
		if err != nil {
			return ctx, nil, errors.Wrap(err, "failed to get MAC randomization setting")
		}
		if resp.Enabled {
			ctxRestore := ctx
			ctx, cancel := ctxutil.Shorten(ctx, time.Second)
			_, err := cli.ShillServiceClient.SetMACRandomize(ctx, &wifi.SetMACRandomizeRequest{Enable: false})
			if err != nil {
				return ctx, nil, errors.Wrap(err, "failed to disable MAC randomization")
			}
			// Restore the setting when exiting.
			restore := func() error {
				cancel()
				if _, err := cli.ShillServiceClient.SetMACRandomize(ctxRestore, &wifi.SetMACRandomizeRequest{Enable: true}); err != nil {
					return errors.Wrap(err, "failed to re-enable MAC randomization")
				}
				return nil
			}
			return ctx, restore, nil
		}
	}
	// Not supported or not enabled. No-op for these cases.
	return ctx, func() error { return nil }, nil
}

// SetWifiEnabled persistently enables/disables Wifi via shill.
func (cli *WifiClient) SetWifiEnabled(ctx context.Context, enabled bool) error {
	req := &wifi.SetWifiEnabledRequest{Enabled: enabled}
	_, err := cli.ShillServiceClient.SetWifiEnabled(ctx, req)
	return err
}

// TurnOffBgscan turns off the DUT's background scan, and returns a shortened ctx and a restoring function.
func (cli *WifiClient) TurnOffBgscan(ctx context.Context) (context.Context, func() error, error) {
	ctxForRestoreBgConfig := ctx
	ctx, cancel := ctxutil.Shorten(ctxForRestoreBgConfig, 2*time.Second)

	testing.ContextLog(ctx, "Disable the DUT's background scan")
	bgscanResp, err := cli.ShillServiceClient.GetBgscanConfig(ctx, &empty.Empty{})
	if err != nil {
		return ctxForRestoreBgConfig, nil, err
	}
	oldBgConfig := bgscanResp.Config

	turnOffBgConfig := *bgscanResp.Config
	turnOffBgConfig.Method = shillconst.DeviceBgscanMethodNone
	if _, err := cli.ShillServiceClient.SetBgscanConfig(ctx, &wifi.SetBgscanConfigRequest{Config: &turnOffBgConfig}); err != nil {
		return ctxForRestoreBgConfig, nil, err
	}

	return ctx, func() error {
		cancel()
		testing.ContextLog(ctxForRestoreBgConfig, "Restore the DUT's background scan config: ", oldBgConfig)
		_, err := cli.ShillServiceClient.SetBgscanConfig(ctxForRestoreBgConfig, &wifi.SetBgscanConfigRequest{Config: oldBgConfig})
		return err
	}, nil
}

// SetWakeOnWifiOption is the type of options of SetWakeOnWifi method of TestFixture.
type SetWakeOnWifiOption func(*wifi.WakeOnWifiConfig)

// WakeOnWifiFeatures returns a option for SetWakeOnWifi to modify the
// WakeOnWiFiFeaturesEnabled property.
func WakeOnWifiFeatures(features string) SetWakeOnWifiOption {
	return func(config *wifi.WakeOnWifiConfig) {
		config.Features = features
	}
}

// WakeOnWifiNetDetectScanPeriod returns an option for SetWakeOnWifi to modify
// the NetDetectScanPeriodSeconds property.
func WakeOnWifiNetDetectScanPeriod(seconds uint32) SetWakeOnWifiOption {
	return func(config *wifi.WakeOnWifiConfig) {
		config.NetDetectScanPeriod = seconds
	}
}

// SetWakeOnWifi sets properties related to wake on WiFi.
func (cli *WifiClient) SetWakeOnWifi(ctx context.Context, ops ...SetWakeOnWifiOption) (shortenCtx context.Context, cleanupFunc func() error, retErr error) {
	resp, err := cli.ShillServiceClient.GetWakeOnWifi(ctx, &empty.Empty{})
	if err != nil {
		return ctx, nil, errors.Wrap(err, "failed to get WoWiFi setting")
	}

	origConfig := resp.Config
	newConfig := *origConfig // Copy so we won't modify the original one.

	// Allow WakeOnWiFi.
	newConfig.Allowed = true
	for _, op := range ops {
		op(&newConfig)
	}

	ctxRestore := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	req := &wifi.SetWakeOnWifiRequest{
		Config: &newConfig,
	}
	if _, err := cli.ShillServiceClient.SetWakeOnWifi(ctx, req); err != nil {
		return ctx, nil, errors.Wrap(err, "failed to set WoWiFi features")
	}
	restore := func() error {
		cancel()
		req := &wifi.SetWakeOnWifiRequest{
			Config: origConfig,
		}
		if _, err := cli.ShillServiceClient.SetWakeOnWifi(ctxRestore, req); err != nil {
			return errors.Wrapf(err, "failed to restore WoWiFi features to %v", origConfig)
		}
		return nil
	}
	return ctx, restore, nil
}

// WatchDarkResume is a wrapper for the streaming gRPC call WatchDarkResume, which
// watches dark resumes before next full resume.
// It returns a function that waits for the response of the gRPC call.
func (cli *WifiClient) WatchDarkResume(ctx context.Context) (func() (*wifi.WatchDarkResumeResponse, error), error) {
	stream, err := cli.ShillServiceClient.WatchDarkResume(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}

	s, err := stream.Recv()
	if err != nil {
		return nil, errors.New("failed to get the ready signal from WatchDarkResume")
	}
	if s.Count != 0 {
		return nil, errors.Errorf("unexpected ready signal=%v", s)
	}
	return func() (*wifi.WatchDarkResumeResponse, error) {
		resp, err := stream.Recv()
		if err != nil {
			return nil, errors.Wrap(err, "failed to receive from WatchDarkResume")
		}
		return resp, nil
	}, nil
}
