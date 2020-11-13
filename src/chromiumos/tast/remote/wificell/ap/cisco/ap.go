// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cisco

import (
	"context"
	"fmt"
	"net"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// AccessPoint is the handle object of an instance of Cisco access point
// within a Cisco wireless controller domain.
// It implements the wificell.APIface interface.
type AccessPoint struct {
	ctrl      *Controller
	conf      *hostapd.Config
	wlanConf  *wlanConfig
	groupName string
	apData    *apData
}

// StartCiscoAP configures an AP instance in Cisco controller domain.
func StartCiscoAP(ctx context.Context, ctrl *Controller, conf *hostapd.Config) (_ *AccessPoint, retErr error) {
	ctx, st := timing.Start(ctx, "StartCiscoAP")
	defer st.End()

	var ap AccessPoint

	ap.ctrl = ctrl
	ap.conf = conf

	wlan, err := ctrl.findWLAN(ctx, conf.SSID)
	if err != nil {
		return nil, errors.Wrap(err, " failed to search for WLAN "+conf.SSID)
	}
	if wlan == nil {
		wlan = new(wlanConfig)
		wlan.id = 0
		wlan.ssid = conf.SSID
		err := ctrl.createWLAN(ctx, wlan)
		if err != nil {
			return nil, errors.Wrapf(err, " failed to create WLAN %v", wlan)
		}
		err = ctrl.enableWLAN(ctx, wlan.id)
		if err != nil {
			return nil, errors.Wrapf(err, " failed to enable WLAN %v", wlan)
		}
		testing.ContextLog(ctx, "Created WLAN "+wlan.ssid)
	}
	ap.wlanConf = wlan

	ap.apData = ctrl.findUnusedAP(ctx)
	if ap.apData == nil {
		return nil, errors.New("no unused AP connected to the controller")
	}

	// wait in case we're just after a config reset
	testing.ContextLog(ctx, "Waiting for AP online: "+ap.apData.name)
	err = ctrl.waitAPOnline(ctx, ap.apData)
	if err != nil {
		return nil, errors.Wrap(err, " timeout waiting for AP to be up")
	}

	// _, err = ctrl.sendCommand(ctx, fmt.Sprintf("config 802.11a disable %s", ap.apData.name))
	// if err != nil {
	// 	return nil, errors.Wrap(err, " failed to disable AP")
	// }

	_, err = ctrl.sendCommand(ctx, fmt.Sprintf("config 802.11a channel ap %s %d", ap.apData.name, conf.Channel), true)
	if err != nil {
		return nil, errors.Wrap(err, " failed to set up AP channel")
	}

	err = ctrl.sendCommandNoResult(ctx, fmt.Sprintf("config 802.11a enable %s", ap.apData.name))
	if err != nil {
		return nil, errors.Wrap(err, " failed to enable AP")
	}

	_, err = ctrl.sendCommand(ctx, "save config", true)
	if err != nil {
		return nil, errors.Wrap(err, " failed to save config")
	}

	testing.Sleep(ctx, 120*time.Second) // TODO active wait

	return &ap, nil
}

// Config TODO implement
func (ap *AccessPoint) Config() *hostapd.Config {
	return ap.conf
}

// ServerIP returns the IP of router in the subnet of WiFi.
func (ap *AccessPoint) ServerIP() net.IP {
	ip := ap.wlanConf.networkIP
	ip[3] = 1
	return ip
}

// ServerSubnet returns the subnet whose ip has been masked.
func (ap *AccessPoint) ServerSubnet() *net.IPNet {
	return &net.IPNet{IP: ap.wlanConf.networkIP, Mask: ap.wlanConf.netmask}
}

// DeauthenticateClient TODO implement
func (ap *AccessPoint) DeauthenticateClient(ctx context.Context, clientMAC string) error {
	return errors.New("Not implemented")
}

// ChangeSSID TODO implement
func (ap *AccessPoint) ChangeSSID(ctx context.Context, ssid string) error {
	return errors.New("Not implemented")
}

// ChangeSubnetIdx TODO implement
func (ap *AccessPoint) ChangeSubnetIdx(ctx context.Context) (retErr error) {
	return errors.New("Not implemented")
}

// ReserveForStop TODO implement
func (ap *AccessPoint) ReserveForStop(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctx, func() {}
}

// Stop TODO implement
func (ap *AccessPoint) Stop(ctx context.Context) error {
	return nil
}

// StartChannelSwitch TODO implement
func (ap *AccessPoint) StartChannelSwitch(ctx context.Context, count, channel int, opts ...hostapd.CSOption) error {
	return errors.New("Not implemented")
}

// Interface TODO remove
func (ap *AccessPoint) Interface() string {
	return ""
}
