// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"os"
	"path/filepath"

	"chromiumos/tast/local/asan"
	"chromiumos/tast/local/bundles/cros/security/openfds"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenFDs,
		Desc:         "Enforces a whitelist of open file descriptors expected in key processes.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func OpenFDs(s *testing.State) {
	ctx := s.Context()
	onASan, err := asan.Enabled(ctx)
	if err != nil {
		s.Fatal("Failed to detect ASan: ", err)
	}
	if onASan {
		testing.ContextLog(ctx, "Running on ASan; /proc is allowed")
	}

	// Dump a systemwide snapshot of open-fd and process table information
	// into the results directory, to assist with any triage/debug later.
	if err := openfds.DumpFDs(ctx, filepath.Join(s.OutDir(), "proc-fd.txt")); err != nil {
		s.Fatal("Failed to snapshot the FDs: ", err)
	}

	// Test plugin processes.
	ePlugin := []openfds.Expectation{
		{PathPattern: `anon_inode:\[event.*\]`, Modes: []uint32{0700}},
		{PathPattern: `pipe:.*`, Modes: []uint32{0300, 0500}},
		{PathPattern: `socket:.*`, Modes: []uint32{0500, 0700}},
		{PathPattern: `/dev/null`, Modes: []uint32{0500}},
		{PathPattern: `/dev/urandom`, Modes: []uint32{0500, 0700}},
		{PathPattern: `/var/log/chrome/chrome_.*`, Modes: []uint32{0300}},
		{PathPattern: `/var/log/ui/ui.*`, Modes: []uint32{0300, 0700}},
	}
	if onASan {
		// On ASan, allow all fd types and opening /proc
		// TODO(jorgelo): revisit this and potentially remove.
		ePlugin = append(ePlugin,
			openfds.Expectation{PathPattern: `/proc`, Modes: []uint32{0500}})
	}

	pprocs, err := chrome.GetPluginProcesses()
	if err != nil {
		s.Fatal("Failed to obtain Chrome Plugin processes: ", err)
	}
	for _, p := range pprocs {
		openfds.Expect(s, ctx, onASan, &p, ePlugin)
	}

	// Test renderer processes.
	eRenderer := []openfds.Expectation{
		{PathPattern: `/dev/shm/.+`, Modes: []uint32{0500, 0700}},
		{PathPattern: `/opt/google/chrome/.*\.pak`, Modes: []uint32{0500}},
		{PathPattern: `/opt/google/chrome/icudtl.dat`, Modes: []uint32{0500}},

		// These used to be bundled with the Chrome binary.
		// See crbug.com/475170.
		{PathPattern: `/opt/google/chrome/natives_blob.bin`, Modes: []uint32{0500}},
		{PathPattern: `/opt/google/chrome/snapshot_blob.bin`, Modes: []uint32{0500}},

		// Font files can be kept open in renderers
		// for performance reasons.  See crbug.com/452227.
		{PathPattern: `/usr/share/fonts/.*`, Modes: []uint32{0500}},

		// Zero-copy texture uploads. crbug.com/607632.
		{PathPattern: `anon_inode:dmabuf`, Modes: []uint32{0700}},

		// Ad blocking ruleset mmapped in for performance.
		{PathPattern: `/home/chronos/Subresource Filter/Indexed Rules/[0-9]*/[0-9\.]*/Ruleset Data`, Modes: []uint32{0500}},
	}
	eRenderer = append(ePlugin, eRenderer...)

	// Renderers have access to DRM vgem device for graphics tile upload.
	// See crbug.com/537474.
	vgem, err := os.Readlink("/dev/dri/vgem")
	if err == nil {
		eRenderer = append(eRenderer,
			openfds.Expectation{PathPattern: "/dev/dri/" + vgem, Modes: []uint32{0700}})
	}

	rprocs, err := chrome.GetRendererProcesses()
	if err != nil {
		s.Fatal("Failed to obtain Chrome renderer processes: ", err)
	}
	for _, p := range rprocs {
		openfds.Expect(s, ctx, onASan, &p, eRenderer)
	}
}
