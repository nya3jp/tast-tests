// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cisco

import (
	"context"
	"net"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/timing"
)

// AccessPoint is the handle object of an instance of Cisco access point
// within a Cisco wireless controller domain.
// It implements the wificell.APIface interface.
type AccessPoint struct {
	name string
	ctrl *Controller
}

// StartCiscoAP configures an AP instance in Cisco controller domain.
func StartCiscoAP(ctx context.Context, ctrl *Controller, name string, conf *hostapd.Config) (_ *AccessPoint, retErr error) {
	ctx, st := timing.Start(ctx, "StartCiscoAP")
	defer st.End()

	var ap AccessPoint

	ap.name = name
	ap.ctrl = ctrl

	//TODO commands to init the AP

	return &ap, nil
}

// Config TODO implement
func (ap *AccessPoint) Config() *hostapd.Config {
	return nil
}

// ServerIP TODO implement
func (ap *AccessPoint) ServerIP() net.IP {
	return nil
}

// ServerSubnet TODO implement
func (ap *AccessPoint) ServerSubnet() *net.IPNet {
	return nil
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
	return ctx, nil
}

// Stop TODO implement
func (ap *AccessPoint) Stop(ctx context.Context) error {
	return errors.New("Not implemented")
}

// StartChannelSwitch TODO implement
func (ap *AccessPoint) StartChannelSwitch(ctx context.Context, count, channel int, opts ...hostapd.CSOption) error {
	return errors.New("Not implemented")
}

// Interface TODO remove
func (ap *AccessPoint) Interface() string {
	return ""
}
