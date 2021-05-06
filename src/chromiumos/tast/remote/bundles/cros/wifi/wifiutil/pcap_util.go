// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

// ConnectAndCollectPcap sets up a WiFi AP and then asks DUT to connect.
// The path to the packet file and the config of the AP is returned.
// Note: This function assumes that TestFixture spawns Capturer for us.
func ConnectAndCollectPcap(ctx context.Context, tf *wificell.TestFixture, apOps []hostapd.Option) (pcapPath string, apConf *hostapd.Config, err error) {
	// As we'll collect pcap file after APIface and Capturer closed, run it
	// in an inner function so that we can clean up easier with defer.
	capturer, conf, err := func(ctx context.Context) (ret *pcap.Capturer, retConf *hostapd.Config, retErr error) {
		collectFirstErr := func(err error) {
			if retErr == nil {
				ret = nil
				retConf = nil
				retErr = err
			}
			testing.ContextLog(ctx, "Error in connectAndCollectPcap: ", err)
		}

		testing.ContextLog(ctx, "Configuring WiFi to connect")
		ap, err := tf.ConfigureAP(ctx, apOps, nil)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to configure AP")
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				collectFirstErr(errors.Wrap(err, "failed to deconfig AP"))
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		testing.ContextLog(ctx, "Connecting to WiFi")
		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			return nil, nil, err
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				collectFirstErr(errors.Wrap(err, "failed to disconnect"))
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()

		capturer, ok := tf.Capturer(ap)
		if !ok {
			return nil, nil, errors.New("cannot get the capturer from TestFixture")
		}
		return capturer, ap.Config(), nil
	}(ctx)
	if err != nil {
		return "", nil, err
	}
	pcapPath, err = capturer.PacketPath(ctx)
	if err != nil {
		return "", nil, err
	}
	return pcapPath, conf, nil
}

// ScanAndCollectPcap requests active scans and collect pcap file on channel ch.
// Path to the pcap file is returned.
func ScanAndCollectPcap(fullCtx context.Context, tf *wificell.TestFixture, name string, scanCount, ch int) (string, error) {
	action := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Request active scans")
		req := &network.RequestScansRequest{Count: int32(scanCount)}
		if _, err := tf.WifiClient().RequestScans(ctx, req); err != nil {
			return errors.Wrap(err, "failed to trigger active scans")
		}
		return nil
	}
	pcapPath, _, err := CollectPcapForAction(fullCtx, tf.Pcap(), name, ch, action)
	return pcapPath, err
}

// CollectPcapForAction starts a capture on the specified channel, performs a
// custom action, and then stops the capture. The path to the pcap file is
// returned. Also return a bool that indicates whether or not the action has
// successfully completed.
func CollectPcapForAction(fullCtx context.Context, router *wificell.Router, name string, ch int, action func(context.Context) error) (string, bool, error) {
	capturer, actionComplete, err := func() (ret *pcap.Capturer, actionComplete bool, retErr error) {
		capturer, err := router.StartCapture(fullCtx, name, ch, nil)
		if err != nil {
			return nil, false, errors.Wrap(err, "failed to start capturer")
		}
		defer func() {
			if err := router.StopCapture(fullCtx, capturer); err != nil {
				if retErr == nil {
					ret = nil
					retErr = errors.Wrap(err, "failed to stop capturer")
				} else {
					testing.ContextLog(fullCtx, "Failed to stop capturer: ", err)
				}
			}
		}()

		ctx, cancel := router.ReserveForStopCapture(fullCtx, capturer)
		defer cancel()

		if err := action(ctx); err != nil {
			return nil, false, err
		}

		return capturer, true, nil
	}()
	if err != nil {
		return "", actionComplete, err
	}
	// Return the path where capturer saves the pcap.
	pcapPath, err := capturer.PacketPath(fullCtx)
	return pcapPath, actionComplete, err
}
