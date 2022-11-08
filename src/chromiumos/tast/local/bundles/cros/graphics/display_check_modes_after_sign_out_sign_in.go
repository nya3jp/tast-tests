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
	connSpec := net.JoinHostPort(chamIP, chamPort)

	// Connect to Chameleon
	ch, err := chameleon.NewChameleond(ctx, connSpec)
	if err != nil {
		s.Fatal("Failed to connect to Chameleon: ", err)
	}

	s.Log("Connected to Chameleon")
	supPorts, err := ch.GetSupportedPorts(ctx)
	if err != nil {
		s.Fatal("Failed to get supported ports: ", err)
	}

	for _, port := range supPorts {

		isPlug, err := ch.IsPlugged(ctx, port)
		if err != nil {
			s.Fatal("Failed to check if port is plugged: ", err)
		}

		if !isPlug {
			continue
		}

		err = ch.Plug(ctx, port)
		if err != nil {
			s.Fatal("Failed to plug in a physically plugged port: ", err)
		}

		s.Log("Port: ", port, " isPlug: ", isPlug)

		cr, err := chrome.New(ctx,
			chrome.NoLogin(),
			chrome.KeepState(),
			chrome.SkipForceOnlineSignInForTesting())

		// Get resolution before logging into Chrome.
		widthBefLogin, heightBefLogin, err := ch.DetectResolution(ctx, port)
		if err != nil {
			s.Fatal("Failed to detect resolution: ", err)
		}
		s.Log(widthBefLogin, heightBefLogin)

		// Log in to Chrome.
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		cr, err = chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		defer cr.Close(cleanupCtx)

		// Get resolution after logging in for comparison.
		widthAftLogin, heightAftLogin, err := ch.DetectResolution(ctx, port)
		if err != nil {
			s.Fatal("Failed to detect resolution: ", err)
		}
		s.Log(widthAftLogin, heightAftLogin)

		// Check chameleon resolution after login.
		if widthBefLogin != widthAftLogin && heightBefLogin != heightAftLogin {
			s.Fatal("Failed as resolution has changed after login")
		}

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to get test API connection: ", err)
		}

		// Log out of Chrome.
		if err = quicksettings.SignOut(ctx, tconn); err != nil {
			s.Fatal("Failed to logout: ", err)
		}

		// Get resolution after logging out for comparison.
		widthAftLogout, heightAftLogout, err := ch.DetectResolution(ctx, port)
		if err != nil {
			s.Fatal("Failed to detect resolution: ", err)
		}

		// Check chameleon resolution after logging out.
		if widthAftLogin != widthAftLogout && heightAftLogin != heightAftLogout {
			s.Error("Failed as resolution has changed after logout")
		}
	}
}
