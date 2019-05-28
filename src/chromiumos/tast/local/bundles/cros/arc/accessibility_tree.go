// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

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

// readTree reads the tree specified by treePath and returns it as a string.
func readTree(treePath string) (string, error) {
	wantTree, err := ioutil.ReadFile(treePath)
	if err != nil {
		return "", err
	}
	return string(wantTree), nil
}

// getDesktopSubtree returns the tree of the current ChromeVox focus.
func getDesktopSubtree(ctx context.Context, chromeVoxConn *chrome.Conn) (string, error) {
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

// Copmpares two subtrees, and if any, writes the diff to a file.
func compareSubtrees(wantSubtree, gotSubtree, outputFilePath string) error {
	if diff := cmp.Diff(wantSubtree, gotSubtree); diff != "" {
		// Write diff output to file, and return error.
		if err := ioutil.WriteFile(outputFilePath, []byte(diff), 0644); err != nil {
			return errors.Wrapf(err, "failed to write to %v", outputFilePath)
		}
		return errors.Errorf("accessibility tree was not as expected, wrote tree diff to %q", outputFilePath)
	}
	return nil
}

// containsSubtree checks if wantTree is contained in gotTree.
func containsSubtree(wantTree, gotTree, outputFilePath string) error {
	splitTree := strings.Split(wantTree, "\n")
	top := splitTree[0]
	gotSubtree, ok := extractSubtree(gotTree, top)
	if !ok {
		return errors.Errorf("subtree is not as expected: %s, subtree is: %s", wantTree, gotTree)
	}
	return compareSubtrees(strings.Join(splitTree[1:], "\n"), gotSubtree, outputFilePath)
}

// extractSubtree extracts a subtree from a tree whose top-level element matches top.
func extractSubtree(gotTree, treeHeader string) (string, bool) {
	// Extract the component, which is under treeHeader.
	splitTree := regexp.MustCompile(regexp.QuoteMeta(treeHeader)+".*").Split(gotTree, -1)
	if len(splitTree) < 2 {
		return "", false
	}

	// Last line in tree contains the indentation of the application tree.
	tree := strings.SplitAfter(splitTree[0], "\n")
	topIndentation := tree[len(tree)-1]
	treeDepth := strings.Count(topIndentation, "+")
	appTree := strings.SplitAfter(splitTree[1], "\n")
	var result []string

	for _, line := range appTree {
		if strings.TrimSpace(line) != "" {
			if strings.Count(line, "+") <= treeDepth {
				break;
			}
			result = append(result, strings.Replace(line, topIndentation, "", 1))
		}
	}

	// A sample line in the obtained accessibility tree is structured as follows:
	// ++++++++++++++++++++++++++++++++++++name=seekBar role=slider location=(6, 184) size=(1188, 24)
	// In the test, we are only concerned with node depth (specified by number of leading +'s)
	// and name/ role.
	roleRe := regexp.MustCompile(`(role=\S+).*`)
	cleanTree := roleRe.ReplaceAllString(strings.Join(result, ""), "$1")
	return cleanTree, true
}

// checkAccessibilityTree checks that accessibility tree for current application,
// matches the expected tree, which is provided in the file specified by wantFilePath.
func checkAccessibilityTree(ctx context.Context, chromeVoxConn *chrome.Conn, wantFilePath, outputFilePath string) error {
	// Read expected tree from input file.
	wantTree, err := readTree(wantFilePath)
	if err != nil {
		return errors.Wrap(err, "could not get tree from file")
	}

	// Obtain the tree of which ChromeVox has focus.
	gotTree, err := getDesktopSubtree(ctx, chromeVoxConn)
	if err != nil {
		return err
	}

	// Compare the current tree with expected tree, and write
	// diff to outputFilePath.
	if err := containsSubtree(wantTree, gotTree, outputFilePath); err != nil {
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
