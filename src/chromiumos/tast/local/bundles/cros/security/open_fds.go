// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/asan"
	"chromiumos/tast/local/bundles/cros/security/openfds"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenFDs,
		Desc: "Enforces a whitelist of open file descriptors expected in key processes",
		Contacts: []string{
			"jorgelo@chromium.org",  // Security team
			"hidehiko@chromium.org", // Tast port author
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func OpenFDs(ctx context.Context, s *testing.State) {
	onASan, err := asan.Enabled(ctx)
	if err != nil {
		s.Fatal("Failed to detect ASan: ", err)
	}
	if onASan {
		testing.ContextLog(ctx, "Running on ASan; /proc is allowed")
	}

	// Log out to clean up any stale FDs that might have been left behind by
	// things that the previous test did: https://crbug.com/924893
	upstart.RestartJob(ctx, "ui")

	// Wait for the renderer processes to fire up.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		rprocs, err := chrome.GetRendererProcesses()
		if err != nil {
			return errors.Wrap(err, "failed to obtain Chrome renderer processes")
		}

		if len(rprocs) == 0 {
			return errors.Wrap(err, "no renderer processes found")
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Error getting renderer processes: ", err)
	}

	// Dump a systemwide snapshot of open-fd and process table information
	// into the results directory, to assist with any triage/debug later.
	if err := openfds.DumpFDs(ctx, filepath.Join(s.OutDir(), "proc-fd.txt")); err != nil {
		s.Fatal("Failed to snapshot the FDs: ", err)
	}

	mkExp := func(pathPattern string, modes ...uint32) openfds.Expectation {
		return openfds.Expectation{PathPattern: pathPattern, Modes: modes}
	}

	// Test plugin processes.
	ePlugin := []openfds.Expectation{
		mkExp(`anon_inode:\[event.*\]`, 0700),
		mkExp(`pipe:.*`, 0300, 0500),
		mkExp(`socket:.*`, 0500, 0700),
		mkExp(`/dev/null`, 0500),
		mkExp(`/dev/urandom`, 0500, 0700),
		mkExp(`/var/log/chrome/chrome_.*`, 0300),
		mkExp(`/var/log/ui/ui.*`, 0300, 0700),
	}
	if onASan {
		// On ASan, allow all fd types and opening /proc
		// TODO(jorgelo): revisit this and potentially remove.
		ePlugin = append(ePlugin, mkExp(`/proc`, 0500))
	}

	pprocs, err := chrome.GetPluginProcesses()
	if err != nil {
		s.Fatal("Failed to obtain Chrome Plugin processes: ", err)
	}
	for _, p := range pprocs {
		openfds.Expect(ctx, s, onASan, &p, ePlugin)
	}

	// Test renderer processes.
	eRenderer := []openfds.Expectation{
		mkExp(`/dev/shm/.+`, 0500, 0700),
		mkExp(`/opt/google/chrome/.*\.pak`, 0500),
		mkExp(`/opt/google/chrome/icudtl.dat`, 0500),

		// These used to be bundled with the Chrome binary.
		// See crbug.com/475170.
		mkExp(`/opt/google/chrome/natives_blob.bin`, 0500),
		mkExp(`/opt/google/chrome/snapshot_blob.bin`, 0500),

		// Font files can be kept open in renderers
		// for performance reasons.  See crbug.com/452227.
		mkExp(`/usr/share/fonts/.*`, 0500),

		// Zero-copy texture uploads. crbug.com/607632.
		mkExp(`anon_inode:dmabuf`, 0700),

		// Ad blocking ruleset mmapped in for performance.
		mkExp(`/home/chronos/Subresource Filter/Indexed Rules/[0-9]*/[0-9\.]*/Ruleset Data`, 0500),

		// Dictionaries.
		mkExp(`/home/chronos/Dictionaries/.*\.bdic`, 0500),
	}
	eRenderer = append(ePlugin, eRenderer...)

	// Renderers have access to DRM vgem device for graphics tile upload.
	// See crbug.com/537474.
	vgem, err := os.Readlink("/dev/dri/vgem")
	if err == nil {
		eRenderer = append(eRenderer, mkExp("/dev/dri/"+vgem, 0700))
	}

	rprocs, err := chrome.GetRendererProcesses()
	if err != nil {
		s.Fatal("Failed to obtain Chrome renderer processes: ", err)
	}
	if len(rprocs) == 0 {
		s.Fatal("No Chrome renderer processes found")
	}

	for _, p := range rprocs {
		openfds.Expect(ctx, s, onASan, &p, eRenderer)
	}
}
