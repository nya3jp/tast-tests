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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// simpleAutomationNode represents the node of accessibilityTree we can obtain from ChromeVox LogStore.
// Defined in https://cs.chromium.org/chromium/src/chrome/browser/resources/chromeos/chromevox/cvox2/background/tree_types.js
type simpleAutomationNode struct {
	Name     string                  `json:"name,omitempty"`
	Role     string                  `json:"role,omitempty"`
	Children []*simpleAutomationNode `json:"children,omitempty"`
	// There are other variables (url, location, value and logStr).
	// They will not be used in the test and thus not included here.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityTree,
		Desc:         "Checks that Chrome accessibility tree for ARC application is correct",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Data:         []string{accessibility.ApkName, "accessibility_tree_expected.json"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

// getExpectedTree returns the accessibility tree read from the specified file.
func getExpectedTree(filepath string) (*simpleAutomationNode, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var root simpleAutomationNode
	err = json.NewDecoder(f).Decode(&root)
	return &root, err
}

// getDesktopTree returns the accessibility tree of the whole desktop.
func getDesktopTree(ctx context.Context, chromeVoxConn *chrome.Conn) (*simpleAutomationNode, error) {
	var root simpleAutomationNode
	const script = `
		new Promise((resolve, reject) => {
			chrome.automation.getDesktop((root) => {
				LogStore.getInstance().writeTreeLog(new TreeDumper(root));
				const logTree = LogStore.instance.getLogsOfType(LogStore.LogType.TREE);
				resolve(logTree[0].logTree_.rootNode);
			});
		})`
	err := chromeVoxConn.EvalPromise(ctx, script, &root)
	return &root, err
}

// findNode recursively finds the node with specified name and role.
func findNode(node *simpleAutomationNode, name, role string) (*simpleAutomationNode, bool) {
	if node.Name == name && node.Role == role {
		return node, true
	}
	for _, ch := range node.Children {
		if ret, found := findNode(ch, name, role); found {
			return ret, true
		}
	}
	return nil, false
}

// dumpTree writes the given accessibility tree to the file specified by filepath.
func dumpTree(tree *simpleAutomationNode, filepath string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(&tree)
	return err
}

func AccessibilityTree(ctx context.Context, s *testing.State) {
	const (
		expectedTreeFile = "accessibility_tree_expected.json"
		actualTreeFile   = "accessibility_tree_actual.json"
		diffFile         = "accessibility_tree_diff_tree_output.txt"
	)

	accessibility.RunTest(ctx, s, func(a *arc.ARC, chromeVoxConn *chrome.Conn, ew *input.KeyboardEventWriter) {
		outFilePath := filepath.Join(s.OutDir(), actualTreeFile)
		diffFilePath := filepath.Join(s.OutDir(), diffFile)

		// Parse expected tree.
		expected, err := getExpectedTree(s.DataPath(expectedTreeFile))
		if err != nil {
			s.Fatal("Failed to get the expected accessibility tree from the file: ", err)
		}

		// Extract accessibility tree.
		root, err := getDesktopTree(ctx, chromeVoxConn)
		if err != nil {
			s.Fatal("Failed to get the actual accessibility tree for current desktop: ", err)
		}

		// Find the root node of Android application.
		appRoot, ok := findNode(root, expected.Name, expected.Role)
		if appRoot == nil || !ok {
			// When the root could not be found, dump the entire tree.
			if err := dumpTree(root, outFilePath); err != nil {
				s.Fatal("Failed to get Android application root from accessibility tree, and dumpTree failed: ", err)
			}
			s.Fatalf("Failed to get Android application root from accessibility tree, wrote the entire tree to %q", actualTreeFile)
		}

		if diff := cmp.Diff(appRoot, expected, cmpopts.EquateEmpty()); diff != "" {
			// When the accessibility tree is different, dump the diff and the obtained tree.
			if err := ioutil.WriteFile(diffFilePath, []byte(diff), 0644); err != nil {
				s.Fatal("Accessibility tree did not match; failed to write diff: ", err)
			}
			if err := dumpTree(appRoot, outFilePath); err != nil {
				s.Fatal("Accessibility tree did not match; failed to dump tree: ", err)
			}
			s.Fatalf("Accessibility tree did not match (see diff:%s, actual:%s)", diffFile, actualTreeFile)
		}
	})
}
