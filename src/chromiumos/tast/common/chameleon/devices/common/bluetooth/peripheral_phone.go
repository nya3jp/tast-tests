// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import "chromiumos/tast/common/xmlrpc"

// PhonePeripheral is an interface for making RPC calls to a chameleond daemon
// targeting a specific bluetooth phone peripheral chameleon device flow.
//
// This is based off of the Python class "chameleond.devices.raspi_bluetooth_flow.RaspiBLEPhone"
// from the chameleon source. Refer to that source for more complete
// documentation.
type PhonePeripheral interface {
	BluezPeripheral
}

// CommonPhonePeripheral is a base implementation of PhonePeripheral that
// provides methods for making XMLRPC calls to a chameleond daemon.
// See the PhonePeripheral interface for more detailed documentation.
type CommonPhonePeripheral struct {
	CommonBluezPeripheral
}

// NewCommonPhonePeripheral creates a new instance of CommonPhonePeripheral.
func NewCommonPhonePeripheral(xmlrpcClient *xmlrpc.XMLRpc, methodNamePrefix string) *CommonPhonePeripheral {
	return &CommonPhonePeripheral{
		CommonBluezPeripheral: *NewCommonBluezPeripheral(xmlrpcClient, methodNamePrefix),
	}
}
