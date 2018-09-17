// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"chromiumos/tast/local/asan"
	"chromiumos/tast/local/bundles/cros/security/openfds"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"os"

	"path/filepath"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenFds,
		Desc:         "Enforces a whitelist of open file descriptors expected in key processes.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func OpenFds(s *testing.State) {
	ctx := s.Context()
	onAsan, err := asan.RunningOnAsan(ctx)
	if err != nil {
		s.Fatal("Failed to detect Asan: ", err)
	}
	if onAsan {
		testing.ContextLog(ctx, "Running on Asan. /proc is allowed.")
	}

	// Dump a systemwide snapshot of open-fd and process table information
	// into the results directory, to assist with any triage/debug later.
	if err := openfds.DumpFds(ctx, filepath.Join(s.OutDir(), "proc-fd.txt")); err != nil {
		s.Fatal("Failed to snapshot the fds: ", err)
	}

	// Test plugin processes.
	ePlugin := []openfds.Expectation{
		{`anon_inode:\[event.*\]`, []uint32{0700}},
		{`pipe:.*`, []uint32{0300, 0500}},
		{`socket:.*`, []uint32{0500, 0700}},
		{`/dev/null`, []uint32{0500}},
		{`/dev/urandom`, []uint32{0500, 0700}},
		{`/var/log/chrome/chrome_.*`, []uint32{0300}},
		{`/var/log/ui/ui.*`, []uint32{0300, 0700}},
	}
	if onAsan {
		// On ASan, allow all fd types and opening /proc
		// TODO(jorgelo): revisit this and potentially remove.
		ePlugin = append(ePlugin,
			openfds.Expectation{`/proc`, []uint32{0500}})
	}

	pprocs, err := chrome.GetPluginProcesses()
	if err != nil {
		s.Fatal("Failed to obtain Chrome Plugin processes: ", err)
	}
	for _, p := range pprocs {
		openfds.Expect(s, ctx, onAsan, &p, ePlugin)
	}

	// Test rendere processes.
	eRenderer := []openfds.Expectation{
		{`/dev/shm/..*`, []uint32{0500, 0700}},
		{`/opt/google/chrome/.*\.pak`, []uint32{0500}},
		{`/opt/google/chrome/icudtl.dat`, []uint32{0500}},

		// These used to be bundled with the Chrome binary.
		// See crbug.com/475170.
		{`/opt/google/chrome/natives_blob.bin`, []uint32{0500}},
		{`/opt/google/chrome/snapshot_blob.bin`, []uint32{0500}},

		// Font files can be kept open in renderers
		// for performance reasons.  See crbug.com/452227.
		{`/usr/share/fonts/.*`, []uint32{0500}},

		// Zero-copy texture uploads. crbug.com/607632.
		{`anon_inode:dmabuf`, []uint32{0700}},

		// Ad blocking ruleset mmapped in for performance.
		{`/home/chronos/Subresource Filter/Indexed Rules/[0-9]*/[0-9\.]*/Ruleset Data`, []uint32{0500}},
	}
	eRenderer = append(ePlugin, eRenderer...)

	// Renderers have access to DRM vgem device for graphics tile upload.
	// See crbug.com/537474.
	vgem, err := os.Readlink("/dev/dri/vgem")
	if err == nil {
		eRenderer = append(eRenderer,
			openfds.Expectation{"/dev/dri/" + vgem, []uint32{0700}})
	}

	rprocs, err := chrome.GetRendererProcesses()
	if err != nil {
		s.Fatal("Failed to obtain Chrome renderer processes: ", err)
	}
	for _, p := range rprocs {
		openfds.Expect(s, ctx, onAsan, &p, eRenderer)
	}
}
