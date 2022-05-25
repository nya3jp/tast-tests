// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/veth"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillVeth,
		Desc: "Verifies a veth pair creates a Device in Shill",
		Contacts: []string{
			"cros-network-health@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// ShillVeth sets up a virtual Ethernet pair and ensures that a corresponding
// Shill Device and Service is created.
func ShillVeth(ctx context.Context, s *testing.State) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy")
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
		vethIface = "test_ethernet"
		// NB: Shill explicitly avoids managing interfaces whose name is prefixed with 'veth'.
		hostapdIface = "test_veth"
	)
	vEth, err := veth.NewPair(ctx, vethIface, hostapdIface)
	if err != nil {
		s.Fatal("Failed to create veth pair")
	} else {
		defer func() {
			if e := vEth.Delete(ctx); e != nil {
				testing.ContextLog(ctx, "Failed to cleanup veth: ", e)
			}
		}()
	}

	d, err := m.WaitForDeviceByName(ctx, vEth.Iface.Name, time.Second*3)
	if err != nil {
		s.Fatal("Failed to find veth device managed by Shill")
	}

	// Get the veth Device and Service properties.
	deviceProps, err := d.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Device properties")
	}
	selectedService, err := deviceProps.Get(shillconst.DevicePropertySelectedService)
	if err != nil {
		s.Fatal("Failed to get Devuce,SelectedService: ", err)
	}
	service, err := shill.NewService(ctx, selectedService.(dbus.ObjectPath))
	if err != nil {
		s.Fatal("Failed to get Service: ", err)
	}
	serviceProps, err := service.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Service properties")
	}
	state, err := serviceProps.GetString(shillconst.ServicePropertyState)
	if err != nil {
		s.Fatal("Failed to get Service.State: ", err)
	}
	if state != shillconst.ServiceStateConfiguration {
		s.Errorf("Unexpected Service.State: %v, %v", state, err)
	}

	// NOTE: service is stuck in a 'Configuration' state at this point.
	// if err := service.WaitForConnectedOrError(ctx); err != nil {
	// 	s.Fatal("Service not connected: ", err)
	// }

	// Prioritize the veth Service. Note: This does not trigger a connect.
	if err = service.SetProperty(ctx, shillconst.ServicePropertyPriority, 1); err != nil {
		s.Fatal("Failed to set Priority: ", err)
	}
}
