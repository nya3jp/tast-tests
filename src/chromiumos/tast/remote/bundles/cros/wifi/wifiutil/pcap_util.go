// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"bytes"
	"context"
	"net"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

// VerifyMACUsedForScan forces Scan, collects the pcap and checks for
// MAC address used in Probe Requests.  If the randomize is turned on
// none of the macs should be used, and if it is turned off then all
// of the Probes should be using MAC from the first element.
func VerifyMACUsedForScan(ctx context.Context, tf *wificell.TestFixture, ap *wificell.APIface,
	name string, randomize bool, macs []net.HardwareAddr) (retErr error) {
	resp, err := tf.WifiClient().SetMACRandomize(ctx, &wifi.SetMACRandomizeRequest{Enable: randomize})
	if err != nil {
		return errors.Wrapf(err, "failed to set MAC randomization to: %t", randomize)
	}
	if resp.OldSetting != randomize {
		testing.ContextLog(ctx, "Switched MAC randomization for scans to: ", randomize)
		// Always restore the setting on leaving.
		defer func(ctx context.Context, restore bool) {
			if _, err := tf.WifiClient().SetMACRandomize(ctx, &wifi.SetMACRandomizeRequest{Enable: restore}); err != nil {
				retErr = errors.Wrapf(err, "failed to restore MAC randomization setting back to %t", restore)
			}
		}(ctx, resp.OldSetting)
	}
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	// Wait for the current scan to be done (if in progress) to avoid
	// possible scan started before our setting.
	if _, err := tf.WifiClient().WaitScanIdle(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to wait for current scan to be done")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	pcapPath, err := ScanAndCollectPcap(timeoutCtx, tf, name, 5, ap.Config().Channel)
	if err != nil {
		return errors.Wrap(err, "failed to collect pcap")
	}

	testing.ContextLog(ctx, "Start analyzing pcap")
	filters := []pcap.Filter{
		pcap.RejectLowSignal(),
		pcap.Dot11FCSValid(),
		pcap.TypeFilter(
			layers.LayerTypeDot11MgmtProbeReq,
			func(layer gopacket.Layer) bool {
				ssid, err := pcap.ParseProbeReqSSID(layer.(*layers.Dot11MgmtProbeReq))
				if err != nil {
					testing.ContextLogf(ctx, "Skipped malformed probe request %v: %v", layer, err)
					return false
				}
				// Take the ones with wildcard SSID or SSID of the AP.
				if ssid == "" || ssid == ap.Config().SSID {
					return true
				}
				return false
			},
		),
	}
	packets, err := pcap.ReadPackets(pcapPath, filters...)
	if err != nil {
		return errors.Wrap(err, "failed to read packets")
	}
	if len(packets) == 0 {
		return errors.New("no probe request found in pcap")
	}
	testing.ContextLogf(ctx, "Total %d probe requests found", len(packets))

	for _, p := range packets {
		// Get sender address.
		layer := p.Layer(layers.LayerTypeDot11)
		if layer == nil {
			return errors.Errorf("ProbeReq packet %v does not have Dot11 layer", p)
		}
		dot11, ok := layer.(*layers.Dot11)
		if !ok {
			return errors.Errorf("Dot11 layer output %v not *layers.Dot11", p)
		}
		sender := dot11.Address2

		if randomize {
			// In this case we are checking if MAC from probe does not
			// match any previously known (given in `macs` argument).
			for _, mac := range macs {
				if bytes.Equal(sender, mac) {
					return errors.New("Found a probe request with a known MAC: " + mac.String())
				}
			}
		} else if !bytes.Equal(sender, macs[0]) {
			return errors.Errorf("found a probe request with a different MAC: got %s, want %s", sender, macs[0])
		}
	}
	return nil
}

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
	p, err := tf.StandardPcap()
	if err != nil {
		return "", errors.Wrap(err, "unable to get standard pcap device")
	}
	return CollectPcapForAction(fullCtx, p, name, ch, nil, action)
}

// CollectPcapForAction starts a capture on the specified channel, performs a
// custom action, and then stops the capture. The path to the pcap file is
// returned.
func CollectPcapForAction(fullCtx context.Context, rt support.Capture, name string, ch int, freqOps []iw.SetFreqOption, action func(context.Context) error) (string, error) {
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
