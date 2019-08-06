// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// simpleAutomationNode represents the node of accessibilityTree we can obtain from ChromeVox LogStore
// Defined in https://cs.chromium.org/chromium/src/chrome/browser/resources/chromeos/chromevox/cvox2/background/tree_dumper.js?l=18
type simpleAutomationNode struct {
	Name     string                 `json:"name"`
	Role     string                 `json:"role"`
	Children []simpleAutomationNode `json:"children"`
	// There are other variables (url, location, value and logStr).
	// We don't use these value in this test and thus ignore them.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityTree,
		Desc:         "Checks that Chrome accessibility tree for ARC application is correct",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"ArcAccessibilityTest.apk", "accessibility_tree_expected.json"},
		Timeout:      4 * time.Minute,
	})
}

// getDesktopTree returns the accessibility tree of the whole desktop.
func getDesktopTree(ctx context.Context, chromeVoxConn *chrome.Conn) (root simpleAutomationNode, err error) {
	const script = `
		new Promise((resolve, reject) => {
			chrome.automation.getDesktop((root) => {
				LogStore.getInstance().writeTreeLog(new TreeDumper(root));
				const logTree = LogStore.instance.getLogsOfType(LogStore.LogType.TREE);
				resolve(logTree[0].logTree_.rootNode);
			});
		})`
	err = chromeVoxConn.EvalPromise(ctx, script, &root)
	return root, err
}

// findNode recursively finds the node with specified name and role.
func findNode(node *simpleAutomationNode, name, role string) *simpleAutomationNode {
	if node.Name == name && node.Role == role {
		return node
	}
	for _, ch := range node.Children {
		ret := findNode(&ch, name, role)
		if ret != nil {
			return ret
		}
	}
	return nil
}

// dumpTree writes the given accessibility tree to the file specified by filepath.
func dumpTree(tree *simpleAutomationNode, filepath string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	bytes, err := json.MarshalIndent(tree, "", "\t")
	if err != nil {
		return err
	}
	f.Write(bytes)

	return nil
}

func AccessibilityTree(ctx context.Context, s *testing.State) {
	const (
		apkName                   = "ArcAccessibilityTest.apk"
		accessibilityTreeExpected = "accessibility_tree_expected.json"
		accessibilityTreeActual   = "accessibility_tree_actual.json"
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

	outFilepath := filepath.Join(s.OutDir(), accessibilityTreeActual)

	// Parse expected tree.
	var expected simpleAutomationNode
	jsonStr, err := ioutil.ReadFile(s.DataPath(accessibilityTreeExpected))
	json.Unmarshal(jsonStr, &expected)

	// Extract accessibility tree.
	root, err := getDesktopTree(ctx, chromeVoxConn)
	if err != nil {
		s.Fatal("Failed to get accessibility tree for current desktop: ", err)
	}

	// Find the root node of Android application.
	appRoot := findNode(&root, expected.Name, expected.Role)
	if appRoot == nil {
		// When the root could not be found, dump the entire tree.
		err := dumpTree(&root, outFilepath)
		if err != nil {
			s.Fatal("Failed to get Android application root from accessibility tree, and dumpTree failed: ", err)
		}
		s.Fatalf("Failed to get Android application root from accessibility tree, wrote the entire tree to %q", outFilepath)
	}

	if !reflect.DeepEqual(appRoot, &expected) {
		// When the accessibility tree is different, dump the obtained tree.
		err := dumpTree(appRoot, outFilepath)
		if err != nil {
			s.Fatal("Accessibility trees are different, and dumpTree failed: ", err)
		}
		s.Fatalf("Accessibility trees are different, wrote the actual tree to %q", outFilepath)
	}
}
