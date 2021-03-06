// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/services/cros/wifi"
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
		req := &wifi.RequestScansRequest{Count: int32(scanCount)}
		if _, err := tf.WifiClient().RequestScans(ctx, req); err != nil {
			return errors.Wrap(err, "failed to trigger active scans")
		}
		return nil
	}
	p, err := tf.LegacyPcap()
	if err != nil {
		return "", errors.Wrap(err, "unable to get legacy pcap device")
	}
	return CollectPcapForAction(fullCtx, p, name, ch, nil, action)
}

// CollectPcapForAction starts a capture on the specified channel, performs a
// custom action, and then stops the capture. The path to the pcap file is
// returned.
func CollectPcapForAction(fullCtx context.Context, rt router.SupportCapture, name string, ch int, freqOps []iw.SetFreqOption, action func(context.Context) error) (string, error) {
	capturer, err := func() (ret *pcap.Capturer, retErr error) {
		capturer, err := rt.StartCapture(fullCtx, name, ch, freqOps)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start capturer")
		}
		defer func() {
			if err := rt.StopCapture(fullCtx, capturer); err != nil {
				if retErr == nil {
					ret = nil
					retErr = errors.Wrap(err, "failed to stop capturer")
				} else {
					testing.ContextLog(fullCtx, "Failed to stop capturer: ", err)
				}
			}
		}()

		ctx, cancel := rt.ReserveForStopCapture(fullCtx, capturer)
		defer cancel()

		if err := action(ctx); err != nil {
			return nil, err
		}

		return capturer, nil
	}()
	if err != nil {
		return "", err
	}
	// Return the path where capturer saves the pcap.
	return capturer.PacketPath(fullCtx)
}
