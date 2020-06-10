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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	axTreeExpectedTreeFilePrefix = "accessibility_tree_expected"
	axTreeActualTreeFilePrefix   = "accessibility_tree_actual"
	axTreeDiffFilePrefix         = "accessibility_tree_diff"
)

// simpleAutomationNode represents the node of accessibilityTree we can obtain from ChromeVox LogStore.
// Defined in https://source.chromium.org/chromium/chromium/src/+/master:chrome/browser/resources/chromeos/accessibility/chromevox/background/logging/tree_dumper.js
// TODO(sarakato): Consider using ui.Node here, as number of tests increase.
type simpleAutomationNode struct {
	Name     string                  `json:"name,omitempty"`
	Role     string                  `json:"role,omitempty"`
	Value    string                  `json:"value,omitempty"`
	Children []*simpleAutomationNode `json:"children,omitempty"`
	// There are other variables (url, location and logStr).
	// They will not be used in the test and thus not included here.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityTree,
		Desc:         "Checks that Chrome accessibility tree for ARC application is correct",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Data:         []string{"accessibility_tree_expected.MainActivity.json", "accessibility_tree_expected.EditTextActivity.json"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
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
func getDesktopTree(ctx context.Context, cvconn *chrome.Conn) (*simpleAutomationNode, error) {
	var root simpleAutomationNode
	err := cvconn.Call(ctx, &root, `async() => {
		  let root = await tast.promisify(chrome.automation.getDesktop)();
		  const instance = LogStore.getInstance();
		  instance.clearLog();
		  instance.writeTreeLog(new TreeDumper(root));
		  const logTree = LogStore.instance.getLogsOfType(LogStore.LogType.TREE);
		  return logTree[0].logTree_.rootNode;
		}`)
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
	testActivities := []accessibility.TestActivity{accessibility.MainActivity, accessibility.EditTextActivity}
	testFunc := func(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn, currentActivity accessibility.TestActivity) error {
		expected, err := getExpectedTree(s.DataPath(axTreeExpectedTreeFilePrefix + currentActivity.Name + ".json"))
		if err != nil {
			return errors.Wrap(err, "failed to get the expected accessibility tree from the file")
		}

		actualFileName := axTreeActualTreeFilePrefix + currentActivity.Name + ".json"
		actualFilePath := filepath.Join(s.OutDir(), actualFileName)

		var appRoot, root *simpleAutomationNode
		// Find the root node of Android application.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Extract accessibility tree.
			root, err = getDesktopTree(ctx, cvconn)
			if err != nil {
				return errors.Wrap(err, "failed to get the actual accessibility tree for current desktop")
			}

			var ok bool
			appRoot, ok = findNode(root, expected.Name, expected.Role)
			if appRoot == nil || !ok {
				return errors.New("failed to get Android root from accessibility tree")
			}

			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			// When the root could not be found, dump the entire tree.
			if dumpTreeErr := dumpTree(root, actualFilePath); dumpTreeErr != nil {
				return errors.Wrapf(dumpTreeErr, "timed out waiting for appRoot and the previous error is %v", err)
			}
			return errors.Wrapf(err, "timed out waiting for appRoot and wrote the entire tree to %q", actualFileName)
		}

		if diff := cmp.Diff(appRoot, expected, cmpopts.EquateEmpty()); diff != "" {
			diffFileName := axTreeDiffFilePrefix + currentActivity.Name + ".txt"
			diffFilePath := filepath.Join(s.OutDir(), diffFileName)
			// When the accessibility tree is different, dump the diff and the obtained tree.
			if err := ioutil.WriteFile(diffFilePath, []byte("(-want +got):\n"+diff), 0644); err != nil {
				return errors.Wrap(err, "accessibility tree did not match; failed to write diff to the file")
			}
			if err := dumpTree(appRoot, actualFilePath); err != nil {
				return errors.Wrap(err, "accessibility tree did not match; failed to dump the actual tree")
			}
			return errors.Errorf("accessibility tree did not match (see diff:%s, actual:%s)", diffFileName, actualFileName)
		}
		return nil
	}
	accessibility.RunTest(ctx, s, testActivities, testFunc)
}
