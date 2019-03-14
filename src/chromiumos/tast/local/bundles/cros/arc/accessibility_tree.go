// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
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
		Desc:         "Checks that Chrome accessibility tree for ARC application is correct",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"accessibility_sample.apk", "accessibility_tree_expected.txt"},
		Timeout:      4 * time.Minute,
	})
}

// readTree reads a tree from an input file, specified by treePath.
func readTree(treePath string) (string, error) {
	wantTree, err := ioutil.ReadFile(treePath)
	if err != nil {
		return "", err
	}
	return string(wantTree), nil
}

// getCurrentTree returns the tree of the current ChromeVox focus.
func getCurrentTree(ctx context.Context, chromeVoxConn *chrome.Conn) (string, error) {
	var gotTree string
	const script = `
		new Promise((resolve, reject) => {
			chrome.automation.getDesktop((root) => {
				LogStore.getInstance().writeTreeLog(new TreeDumper(root));
				const logTree = LogStore.instance.getLogsOfType(TreeLog.LogType.TREE);
				resolve(logTree[0].logTree_.treeToString());
			});
		})`
	if err := chromeVoxConn.EvalPromise(ctx, script, &gotTree); err != nil {
		return "", errors.Wrap(err, "could not get accessibility tree for current focus")
	}
	return gotTree, nil
}

// compareTrees compares wantTree with gotTree, and writes the diff to outputFilePath.
func compareTrees(wantTree, gotTree, outputFilePath string) error {
	wantTreeLines := strings.Split(string(wantTree), "\n")
	wantTreeHeader, wantTreeBody := wantTreeLines[0], wantTreeLines[1:]

	// Check that tree exists, and obtain corressponding component.
	gotTreeLines, err := checkTreeExists(wantTreeHeader, gotTree)
	if err != nil {
		return errors.Wrap(err, "tree does not exist")
	}

	// Parse gotTree, and remove empty entries.
	gotTreeBody := strings.Split(gotTreeLines[1], "\n")
	var gotTreeRemoved []string
	for _, line := range gotTreeBody {
		if ((strings.TrimSpace(line) != "")  && (!strings.Contains(line, "role=ignored"))) {
			gotTreeRemoved = append(gotTreeRemoved, line)
		}
	}

	if len(gotTree) < len(wantTreeBody) {
		return errors.Errorf("current accessibility tree for application does not contain enough elements, got %d; want %d", len(gotTree), len(wantTreeBody))
	}

	// Compute diff of accessibility tree.
	var diff []string
	for i, wantLine := range wantTreeBody {
		if !strings.Contains(gotTreeRemoved[i], wantLine) {
			diff = append(diff, fmt.Sprintf("want %q, got %q\n", string(wantLine), string(gotTreeRemoved[i])))
		}
	}

	// Write diff output to file, and return error.
	if len(diff) > 0 {
		if err := ioutil.WriteFile(outputFilePath, []byte(strings.Join(diff, "\n")), 0644); err != nil {
			return errors.Wrapf(err, "failed to write to %v", outputFilePath)
		}
		return errors.Errorf("accessibility tree was not as expected, wrote tree diff to %q", outputFilePath)
	}
	return nil
}

// checkTreeExists, checks if the tree specified by wantTreeHeader exists,
// and returns the corresponding component of the tree.
func checkTreeExists(wantTreeHeader, gotTree string) ([]string, error) {
	// Extract the component under wantTreeHeader.
	splitTree := regexp.MustCompile(regexp.QuoteMeta(wantTreeHeader)+".*").Split(gotTree, -1)

	if len(splitTree) == 1 {
		return nil, errors.New("Accessibility Sample does not exist inside of tree")
	}
	return splitTree, nil
}

// checkAccessibilityTree checks that accessibility tree for current application,
// matches the expected tree, which is provided in the file specified by wantFilePath.
// The diff (if any), is written to outputFilePath.
func checkAccessibilityTree(ctx context.Context, chromeVoxConn *chrome.Conn, wantFilePath, outputFilePath string) error {
	// Read expected tree from input file.
	wantTree, err := readTree(wantFilePath)
	if err != nil {
		return errors.Wrap(err, "could not get tree from file")
	}

	// Get accessibility tree.
	gotTree, err := getCurrentTree(ctx, chromeVoxConn)
	if err != nil {
		return errors.Wrap(err, "could not tree for current ChromeVox focus")
	}

	// Compare the current tree with expected tree, and write
	// diff to outputFilePath.
	if err := compareTrees(wantTree, gotTree, outputFilePath); err != nil {
		return errors.Wrap(err, "tree was not as expected")
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
