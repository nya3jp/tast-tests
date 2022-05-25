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
			"stevenjb@google.com",
			"cros-network-health-team@google.com",
		},
		Attr:    []string{"group:mainline", "informational"},
		Fixture: "shillReset",
		Timeout: 5 * time.Minute,
	})
}

// ShillVeth sets up a test virtual Ethernet pair and ensures that a corresponding
// Shill Device and Service are created.
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
	restarted := false
	popFunc, err := m.PushTestProfile(ctx)
	if err != nil {
		s.Fatal("Failed to push test profile: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Second)
	defer cancel()
	defer func() {
		if !restarted {
			popFunc()
		} else {
			m.RemoveTestProfile(cleanupCtx)
		}
	}()

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
			if e := vEth.Delete(cleanupCtx); e != nil {
				testing.ContextLog(cleanupCtx, "Failed to cleanup veth: ", e)
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
		s.Error("Failed to get Service.State: ", err)
	} else {
		s.Log("Service.State: ", state)
	}

	initialPri, err := serviceProps.GetInt32(shillconst.ServicePropertyPriority)
	if err != nil {
		s.Fatal("Failed to get Service.Priority: ", err)
	}

	// Prioritize the veth Service. Note: This does not trigger a connect.
	// The priority should be persisted to the temporary profile, not the default profile.
	if err = service.SetProperty(ctx, shillconst.ServicePropertyPriority, initialPri+1); err != nil {
		s.Fatal("Failed to set Priority: ", err)
	}

	if ethService == nil {
		s.Log("No primary Ethernet service, exiting without testing Shill restart")
		return
	}

	// Restart Shill which will ensure that any properties written to the default profile are saved.
	// Note: that should not include the Priority set above.
	// On restart Shill will create a Device for the built-in Ethernet with properties from the default profile.
	s.Log("Restarting Shill")
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		s.Fatal("Failed starting Shill: ", err)
	}
	restarted = true

	// Verify that Ethernet becomes connected.
	s.Log("Waiting for Ethernet Service after Shill restart")
	ethService, err = m.WaitForServiceProperties(ctx, ethProperties, 60*time.Second)
	if err != nil {
		s.Fatal("Failed to get Ethernet Service after Shill restart: ", err)
	}

	// Verify that Ethernet has the initial Priority, not the one set above.
	ethServiceProps, err := ethService.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Ethernet Service properties: ", err)
	}
	priority, err := ethServiceProps.GetInt32(shillconst.ServicePropertyPriority)
	if err != nil {
		s.Fatal("Failed to get Ethernet Service.Priority: ", err)
	}
	if priority != initialPri {
		s.Fatalf("Unexpected Ethernet Service.Priority: %v, %v", priority, err)
	}
}
