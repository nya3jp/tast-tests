// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"
	"time"

	"github.com/godbus/dbus"
	"golang.org/x/sys/unix"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConfigureServiceForProfile,
		Desc: "Test ConfigureServiceForProfile D-Bus method",
		Contacts: []string{
			"matthewmwang@chromium.org",
		},
		Attr: []string{"informational"},
	})
}

func ConfigureServiceForProfile(ctx context.Context, s *testing.State) {
	const (
		filePath   = "/var/cache/shill/default.profile"
		lockPath   = "/run/autotest_pause_ethernet_hook"
		objectPath = dbus.ObjectPath("/profile/default")
	)

	// We lose connectivity along the way here, and if that races with
	// check_ethernet.hook, it may interrupt us.
	lockchan := make(chan error) // To notify lock completion to main thread.
	done := make(chan struct{})  // To notify main thread completion to the goroutine.
	defer close(done)            // Notify thread at end of test.

	go func() {
		f, err := os.Create(lockPath)
		if err != nil {
			lockchan <- err
			return
		}
		defer f.Close()

		if err = unix.Flock(int(f.Fd()), unix.LOCK_SH); err != nil {
			lockchan <- err
			return
		}
		defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
		lockchan <- nil
		<-done // Wait for test completion.
	}()

	lctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	select {
	case err := <-lockchan:
		if err != nil {
			s.Fatalf("Failed to acquire lock %s: %v", lockPath, err)
		}
	case <-lctx.Done():
		s.Fatalf("Timed out acquiring lock %s: %v", lockPath, lctx.Err())
	}

	// Stop shill temporarily and remove the default profile.
	if err := shill.SafeStop(ctx); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	os.Remove(filePath)
	if err := shill.SafeStart(ctx); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Clean up custom services on exit.
	defer func() {
		shill.SafeStop(ctx)
		os.Remove(filePath)
		shill.SafeStart(ctx)
	}()

	props := map[string]interface{}{
		"Type":                 "ethernet",
		"StaticIP.NameServers": "8.8.8.8",
	}
	_, err = manager.ConfigureServiceForProfile(ctx, objectPath, props)
	if err != nil {
		s.Fatal("Unable to configure service: ", err)
	}

	// Restart shill to ensure that configurations persist across reboot.
	if err := shill.SafeStop(ctx); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	if err := shill.SafeStart(ctx); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}
	manager, err = shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := manager.FindMatchingService(ctx, props)
		return err
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Service not found: ", err)
	}
}
