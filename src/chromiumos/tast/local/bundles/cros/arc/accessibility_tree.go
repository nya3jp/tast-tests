// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityTree,
		Desc:         "Checks Chrome accessibility tree for ARC",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"accessibility_sample.apk", "accessibility_tree_expected.txt"},
		Timeout:      4 * time.Minute,
	})
}

func checkAccessibilityTree(ctx context.Context, chromeVoxConn *chrome.Conn, wantFilePath, outputFilePath string) error {
	// Read expected tree from input file.
	wantTree, err := ioutil.ReadFile(wantFilePath)
	if err != nil {
		return err
	}
	wantTreeLines := strings.Split(string(wantTree), "\n")
	wantTreeHeader, wantTreeBody := wantTreeLines[0], wantTreeLines[1:]

	// Get accessibility tree.
	var gotTree string
	const script = `
		new Promise((resolve, reject) => {
			chrome.automation.getDesktop((root) => {
				LogStore.getInstance().writeTreeLog(new TreeDumper(root));
				let logTree = LogStore.instance.getLogsOfType(TreeLog.LogType.TREE);
				resolve(logTree[0].logTree_.treeToString());
			});
		})`
	if err := chromeVoxConn.EvalPromise(ctx, script, &gotTree); err != nil {
		return errors.Wrap(err, "could not get accessibility tree")
	}

	// Remove application line from got, since it has been checked already.
	// Makes it easier to extract the component we want in the tree.
	splitTree := strings.SplitAfter(gotTree, wantTreeHeader)
	if len(splitTree) == 1 {
		return errors.New("Accessibility Sample does not exist inside of tree")
	}

	// Prepare got data, by parsing into array and removing empty entries.
	gotTreeBody := strings.Split(splitTree[1], "\n")
	var gotTreeRemoved []string
	for _, line := range gotTreeBody {
		if strings.TrimSpace(line) != "" {
			gotTreeRemoved = append(gotTreeRemoved, line)
		}
	}

	// Compute diff of accessibility tree.
	var diff []string
	for i, wantLine := range wantTreeBody {
		// Check that want line is contained in gotLine.
		if !strings.Contains(strings.TrimSpace(string(gotTreeRemoved[i])), wantLine) {
			diff = append(diff, fmt.Sprintf("want %q, got %q\n", string(wantLine), string(gotTreeRemoved[i])))
		}
	}

	// Write diff output to file, and return error.
	if len(diff) > 0 {
		if err := ioutil.WriteFile(outputFilePath, []byte(strings.Join(diff, "\n")), 0644); err != nil {
			return errors.Wrapf(err, "failed to write to %v", outputFilePath)
		}
	}
	return nil
}

func AccessibilityTree(ctx context.Context, s *testing.State) {
	const (
		apkName                     = "accessibility_sample.apk"
		accessibilityTreeExpected   = "accessibility_tree_expected.txt"
		accessibilityTreeOutputFile = "accessibility_event_diff_tree_output.txt"
	)
	cr, err := accessibility.NewChrome(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := accessibility.NewARC(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	if err := accessibility.InstallAndStartSampleApp(ctx, a, s.DataPath(apkName)); err != nil {
		s.Fatal("Setting up ARC environment with accessibility failed: ", err)
	}

	if err := accessibility.EnableSpokenFeedback(ctx, cr, a); err != nil {
		s.Fatal("Failed enabling spoken feedback: ", err)
	}

	chromeVoxConn, err := accessibility.ChromeVoxExtConn(ctx, cr)
	if err != nil {
		s.Fatal("Creating connection to ChromeVox extension failed: ", err)
	}
	defer chromeVoxConn.Close()

	// Wait for ChromeVox to stop speaking before interacting with it further.
	if accessibility.WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		s.Fatal("Could not wait for ChromeVox to stop speaking: ", err)
	}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	// Trigger tab event and ensure that accessibility focus dives inside ARC app.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		s.Fatal("Accel(Tab) returned error: ", err)
	}

	// Waiting for element to be focused ensures that contents of ARC accessibility tree has been computed.
	if err := accessibility.WaitForElementFocused(ctx, chromeVoxConn, "android.widget.ToggleButton"); err != nil {
		s.Fatal("Timed out polling for element: ", err)
	}

	// Check accessibility tree is what we expect it to be.
	// This needs to occur after tab event, as the focus from the tab event results in nodes of the accessibility tree to be computed.
	if err := checkAccessibilityTree(ctx, chromeVoxConn, s.DataPath(accessibilityTreeExpected), filepath.Join(s.OutDir(), accessibilityTreeOutputFile)); err != nil {
		s.Fatal("Failed getting accessibility tree, after focus and check: ", err)
	}
}
