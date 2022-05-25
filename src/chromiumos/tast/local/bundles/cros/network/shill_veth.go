// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/veth"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillVeth,
		Desc: "Verifies that a test veth pair creates a Device and Service in Shill",
		Contacts: []string{
			"cros-network-health@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// ShillVeth sets up a test virtual Ethernet pair and ensures that a corresponding
// Shill Device and Service is created.
// Note: This configuration is only used in tests and the veth pair will not by default be connected.
// This also ensures that changes to the veth Device Priority do not affect the Ethernet device.
func ShillVeth(ctx context.Context, s *testing.State) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy")
	}

	var ethProperties = map[string]interface{}{
		shillconst.ServicePropertyType:        shillconst.TypeEthernet,
		shillconst.ServicePropertyIsConnected: true,
	}

	// Check whether Ethernet is connected.
	s.Log("Waiting for initial Ethernet Service")
	ethService, err := m.WaitForServiceProperties(ctx, ethProperties, 5*time.Second)
	if err != nil {
		s.Log("No Ethernet Service: ", err)
	}

	// Set up a test profile.
	popFunc, err := m.PushTestProfile(ctx)
	if err != nil {
		s.Fatal("Failed to push test profile: ", err)
	}
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Second)
	defer cancel()
	defer popFunc()

	// Prepare virtual ethernet link.
	const (
		// Note: Shill does not manage interfaces with names prefixed with 'veth',
		// so use 'test' as a prefix for both of these.
		vethIface = "test_veth"
		peerIface = "test_peer"
	)
	vEth, err := veth.NewPair(ctx, vethIface, peerIface)
	if err != nil {
		s.Fatal("Failed to create veth pair")
	} else {
		defer func() {
			if e := vEth.Delete(ctx); e != nil {
				testing.ContextLog(ctx, "Failed to cleanup veth: ", e)
			}
		}()
	}

	d, err := m.WaitForDeviceByName(ctx, vEth.Iface.Name, 3*time.Second)
	if err != nil {
		s.Fatal("Failed to find veth device managed by Shill")
	}

	service, err := d.WaitForSelectedService(ctx, shillconst.DefaultTimeout)
	if err != nil {
		s.Fatal("Failed to get Service: ", err)
	}
	serviceProps, err := service.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Service properties: ", err)
	}

	state, err := serviceProps.GetString(shillconst.ServicePropertyState)
	if err != nil {
		s.Fatal("Failed to get Service.State: ", err)
	}
	if state != shillconst.ServiceStateConfiguration {
		s.Errorf("Unexpected Service.State: %v, %v", state, err)
	}

	// Prioritize the veth Service. Note: This does not trigger a connect.
	if err = service.SetProperty(ctx, shillconst.ServicePropertyPriority, 1); err != nil {
		s.Fatal("Failed to set Priority: ", err)
	}

	if ethService == nil {
		s.Log("No primary Ethernet service, exiting without testing Shill restart")
		return
	}

	// Restart Shill which will ensure that any profile properties are saved, and
	// will create a Device for the built-in Ethernet with the saved properties.
	s.Log("Restarting Shill")
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		s.Fatal("Failed starting Shill: ", err)
	}

	// Verify that Ethernet becomes connected.
	s.Log("Waiting for Ethernet Service after Shill restart")
	ethService, err = m.WaitForServiceProperties(ctx, ethProperties, 60*time.Second)
	if err != nil {
		s.Fatal("Failed to get Ethernet Service after Shill restart: ", err)
	}

	// Verify that Ethernet is not prioritized.
	ethServiceProps, err := ethService.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Ethernet Service properties: ", err)
	}
	priority, err := ethServiceProps.GetInt32(shillconst.ServicePropertyPriority)
	if err != nil {
		s.Fatal("Failed to get Ethernet Service.Priority: ", err)
	}
	if priority != 0 {
		s.Fatalf("Unexpected Ethernet Service.Priority: %v, %v", priority, err)
	}
}
