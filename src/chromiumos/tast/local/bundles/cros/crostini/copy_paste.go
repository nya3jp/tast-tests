// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

const (
	utf8Data = "Some data that gets copied 🔥 ❄"

	// copyApplet is the data dependency needed to run a copy operation.
	copyApplet      = "copy_applet.py"
	copyAppletTitle = "gtk3_copy_demo"

	// pasteApplet is the data dependency needed to run a paste operation.
	pasteApplet      = "paste_applet.py"
	pasteAppletTitle = "gtk3_paste_demo"
)

// CopyConfig holds the configuration for the copy half of the test.
type copyConfig struct {
	gdkBackend string
	cmdArgs    []string
}

// waylandCopyConfig is the configuration needed to test copying from
// a wayland application.
var waylandCopyConfig = &copyConfig{
	gdkBackend: "wayland",
	cmdArgs:    []string{"env", "GDK_BACKEND=wayland", "python3", copyApplet},
}

// x11CopyConfig is the configuration needed to test copying from
// an X11 application.
var x11CopyConfig = &copyConfig{
	gdkBackend: "x11",
	cmdArgs:    []string{"env", "GDK_BACKEND=x11", "python3", copyApplet},
}

// PasteConfig holds the configuration for the paste half of the test.
type pasteConfig struct {
	gdkBackend string
	cmdArgs    []string
}

// waylandPasteConfig is the configuration needed to test pasting into
// a wayland application.
var waylandPasteConfig = &pasteConfig{
	gdkBackend: "wayland",
	cmdArgs:    []string{"env", "GDK_BACKEND=wayland", "python3", pasteApplet},
}

// x11PasteConfig is the configuration needed to test pasting into
// a x11 application.
var x11PasteConfig = &pasteConfig{
	gdkBackend: "x11",
	cmdArgs:    []string{"env", "GDK_BACKEND=x11", "python3", pasteApplet},
}

// testParameters contains all the data needed to run a single test iteration.
type testParameters struct {
	Copy  *copyConfig
	Paste *pasteConfig
}

func init() {

	testing.AddTest(&testing.Test{
		Func:         CopyPaste,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test copy paste functionality",
		Contacts:     []string{"clumptini+oncall@google.com"},
		Attr:         []string{"group:mainline"},
		Data:         []string{copyApplet, pasteApplet},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			// Parameters generated by copy_paste_test.go. DO NOT EDIT.
			{
				Name:              "wayland_to_wayland_buster_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  waylandCopyConfig,
					Paste: waylandPasteConfig,
				},
			}, {
				Name:              "wayland_to_wayland_buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  waylandCopyConfig,
					Paste: waylandPasteConfig,
				},
			}, {
				Name:              "wayland_to_wayland_bullseye_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  waylandCopyConfig,
					Paste: waylandPasteConfig,
				},
			}, {
				Name:              "wayland_to_wayland_bullseye_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  waylandCopyConfig,
					Paste: waylandPasteConfig,
				},
			}, {
				Name:              "wayland_to_x11_buster_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  waylandCopyConfig,
					Paste: x11PasteConfig,
				},
			}, {
				Name:              "wayland_to_x11_buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  waylandCopyConfig,
					Paste: x11PasteConfig,
				},
			}, {
				Name:              "wayland_to_x11_bullseye_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  waylandCopyConfig,
					Paste: x11PasteConfig,
				},
			}, {
				Name:              "wayland_to_x11_bullseye_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  waylandCopyConfig,
					Paste: x11PasteConfig,
				},
			}, {
				Name:              "x11_to_wayland_buster_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  x11CopyConfig,
					Paste: waylandPasteConfig,
				},
			}, {
				Name:              "x11_to_wayland_buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  x11CopyConfig,
					Paste: waylandPasteConfig,
				},
			}, {
				Name:              "x11_to_wayland_bullseye_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  x11CopyConfig,
					Paste: waylandPasteConfig,
				},
			}, {
				Name:              "x11_to_wayland_bullseye_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  x11CopyConfig,
					Paste: waylandPasteConfig,
				},
			}, {
				Name:              "x11_to_x11_buster_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  x11CopyConfig,
					Paste: x11PasteConfig,
				},
			}, {
				Name:              "x11_to_x11_buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  x11CopyConfig,
					Paste: x11PasteConfig,
				},
			}, {
				Name:              "x11_to_x11_bullseye_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  x11CopyConfig,
					Paste: x11PasteConfig,
				},
			}, {
				Name:              "x11_to_x11_bullseye_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val: testParameters{
					Copy:  x11CopyConfig,
					Paste: x11PasteConfig,
				},
			},
		},
	})
}

func CopyPaste(ctx context.Context, s *testing.State) {
	pre := s.FixtValue().(crostini.FixtureData)
	param := s.Param().(testParameters)
	tconn := pre.Tconn
	cont := pre.Cont

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Copying testing applets to container")
	if err := cont.PushFile(ctx, s.DataPath(copyApplet), copyApplet); err != nil {
		s.Fatal("Failed to push copy applet to container: ", err)
	}
	if err := cont.PushFile(ctx, s.DataPath(pasteApplet), pasteApplet); err != nil {
		s.Fatal("Failed to push paste applet to container: ", err)
	}

	// Add the names of the backends used by each part of the test to differentiate the data used by each test run.
	copiedData := fmt.Sprintf("%v to %v %s", param.Copy.gdkBackend, param.Paste.gdkBackend, utf8Data)

	// The copy event happens at some indeterminate time after the
	// copy applet receives a key press. To be sure we get that event
	// we have to start listening for it before that point.
	// Here, wrapping the promise by a closure in order not to be
	// awaited at this moment.
	var waiting chrome.JSObject
	if err := tconn.Eval(ctx, `(p => () => p)(new Promise((resolve) => {
		  const listener = (e) => {
		    chrome.autotestPrivate.onClipboardDataChanged.removeListener(listener);
		    resolve();
		  };
		  chrome.autotestPrivate.onClipboardDataChanged.addListener(listener);
		}))`, &waiting); err != nil {
		s.Fatal("Failed to set listener for 'copy' event: ", err)
	}
	defer waiting.Release(cleanupCtx)
	if _, err := crostini.RunWindowedApp(ctx, tconn, cont, pre.KB, 120*time.Second, func(ctx context.Context) error {
		// Unwrap the promise to wait its settled state.
		return tconn.Call(ctx, nil, `p => p()`, &waiting)
	}, true, copyAppletTitle, append(param.Copy.cmdArgs, copiedData)); err != nil {
		s.Fatal("Failed to run copy applet: ", err)
	}

	output, err := crostini.RunWindowedApp(ctx, tconn, cont, pre.KB, 30*time.Second, nil, false, pasteAppletTitle, param.Paste.cmdArgs)
	if err != nil {
		s.Fatal("Failed to run paste application: ", err)
	}

	if output != copiedData {
		s.Fatalf("Unexpected paste output: got %q, want %q", output, copiedData)
	}
}
