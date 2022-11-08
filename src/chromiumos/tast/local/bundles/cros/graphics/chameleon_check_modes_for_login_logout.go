// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

var (
	chameleonIP = testing.RegisterVarString(
		"graphics.chameleon_ip",
		"",
		"IP address of Chameleon (required)")

	chameleonPort = testing.RegisterVarString(
		"graphics.chameleon_port",
		"9992",
		"Port for chameleond on Chameleon (optional/used)")
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisplayCheckModesAfterSignOutSignIn,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "To Check the display mode is preserved after sign out and signin",
		Contacts:     []string{"markyacoub@google.com", "chromeos-gfx-display@google.com"},
		VarDeps:      []string{"graphics.chameleon_ip"},
		Attr:         []string{"group:graphics", "graphics_chameleon_igt"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.LoginTimeout + time.Minute,
	})
}

func DisplayCheckModesAfterSignOutSignIn(ctx context.Context, s *testing.State) {

	addr := net.ParseIP(chameleonIP.Value())
	if addr == nil {
		s.Fatal("Failed to get chameleon ip. The Chameleon ip: ", addr)
	}

	chamIP := chameleonIP.Value()
	chamPort := chameleonPort.Value()
	chamURL := net.JoinHostPort(chamIP, chamPort)

	// Connect to Chameleon
	cham, err := chameleon.NewChameleond(ctx, chamURL)
	if err != nil {
		s.Fatal("Failed to connect to Chameleon: ", err)
	}

	s.Log("Connected to Chameleon")
	supportedPorts, err := cham.GetSupportedPorts(ctx)
	if err != nil {
		s.Fatal("Failed to get supported ports: ", err)
	}

	for _, port := range supportedPorts {

		canUsePort, err := shouldUsePort(ctx, cham, port)
		if err != nil {
			s.Fatalf("Failed to determine if plug can be used for port %d: %s", port, err)
		}

		if !canUsePort {
			continue
		}

		err = cham.Plug(ctx, port)
		if err != nil {
			s.Fatalf("Failed to plug in a physically plugged port %d: %s", port, err)
		}

		s.Logf("Starting test on Port %d", port)

		cr, err := chrome.New(ctx,
			chrome.NoLogin(),
			chrome.KeepState(),
			chrome.SkipForceOnlineSignInForTesting())
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}

		preLoginWidth, preLoginHeight, err := cham.DetectResolution(ctx, port)
		if err != nil {
			s.Fatal("Failed to detect prelogin resolution: ", err)
		}
		s.Logf("preLoginWidth = %d and preLoginHeight = %d", preLoginWidth, preLoginHeight)

		// Log in to Chrome.
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		cr, err = chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		defer cr.Close(cleanupCtx)

		postLoginWidth, postLoginHeight, err := cham.DetectResolution(ctx, port)
		if err != nil {
			s.Fatal("Failed to detect post login resolution: ", err)
		}
		s.Logf("postLoginWidth = %d and postLoginHeight = %d", postLoginWidth, postLoginHeight)

		// Check chameleon resolution after login.
		if preLoginWidth != postLoginWidth {
			s.Fatalf("Failed as resolution's width changed after login from %d to %d", preLoginWidth, postLoginWidth)
		}

		if preLoginHeight != postLoginHeight {
			s.Fatalf("Failed as resolution's height changed after login from %d to %d", preLoginHeight, postLoginHeight)
		}

		// Log out of Chrome.
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to get test API connection: ", err)
		}

		if err = quicksettings.SignOut(ctx, tconn); err != nil {
			s.Fatal("Failed to logout: ", err)
		}

		postLogoutWidth, postLogoutHeight, err := cham.DetectResolution(ctx, port)
		if err != nil {
			s.Fatal("Failed to detect resolution: ", err)
		}
		s.Logf("postLogoutWidth = %d and postLogoutHeight = %d", postLogoutWidth, postLogoutHeight)

		// Check chameleon resolution after logging out.
		if postLoginWidth != postLogoutWidth {
			s.Fatalf("Failed as resolution's width changed after logout from %d to %d", postLoginWidth, postLogoutWidth)
		}

		if postLoginHeight != postLogoutHeight {
			s.Fatalf("Failed as resolution's height changed after logout from %d to %d", postLoginHeight, postLogoutHeight)
		}

		err = cham.Unplug(ctx, port)
		if err != nil {
			s.Fatalf("Failed to unplug a physically plugged port %d: %s ", port, err)
		}
	}
}

// shouldUsePort determines whether a port is physically plugged for usage.
func shouldUsePort(ctx context.Context, cham chameleon.Chameleond, port chameleon.PortID) (bool, error) {
	err := cham.Plug(ctx, port)
	if err != nil {
		return false, errors.Wrap(err, "failed to plug in a physically plugged port")
	}

	isPhysPlug, err := cham.IsPhysicalPlugged(ctx, port)
	if err != nil {
		return false, errors.Wrap(err, "failed to check if port is physically plugged")
	}

	err = cham.Unplug(ctx, port)
	if err != nil {
		return false, errors.Wrap(err, "failed to plug in a physically plugged port")
	}
	return isPhysPlug, nil
}
