// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

const (
	axTreeActualTreeFilePrefix = "accessibility_tree_actual"
	axTreeDiffFilePrefix       = "accessibility_tree_diff"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityTree,
		Desc:         "Checks that Chrome accessibility tree for ARC application is correct",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

// getExpectedTree returns the accessibility tree read from the specified file.
func getExpectedTree(filepath string) (*ui.Node, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var root ui.Node
	err = json.NewDecoder(f).Decode(&root)
	return &root, err
}

// dumpTree writes the given accessibility tree to the file specified by filepath.
func dumpTree(tree *ui.Node, filepath string) error {
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

func checkTreeRecursive(ctx context.Context, root *ui.Node, children []ui.Node) (ui.Node, error) {
	for _, node := range children {
		// Create findParams object for non-nil fields.
		var findParams ui.FindParams
		if node.Name != "" {
			findParams.Name = node.Name
		}
		if node.Value != "" {
			findParams.Value = node.Value
		}
		findParams.Role = node.Role

		if childFound, err := root.Descendant(ctx, findParams); err != nil || childFound == nil {
			testing.ContextLogf(ctx, "Could not find %q", node.Name)
			return node, errors.Wrap(err, "failed to find child")
		} else if childFound == nil {
			return node, errors.New("found child, but it is null")
		}

		if len(node.Children) > 0 {
			// Want to do this recursively for each child.
			// Find each child in appRoot.
			return checkTreeRecursive(ctx, root, node.Children)
		}
	}
	return ui.Node{}, nil
}

func AccessibilityTree(ctx context.Context, s *testing.State) {
	MainActivityTree := ui.Node{
		Name: "Main Activity",
		Role: ui.RoleTypeApplication,
		Children: []ui.Node{
			ui.Node{
				Role: ui.RoleTypeGenericContainer,
				Children: []ui.Node{
					ui.Node{
						Name: "Main Activity",
						Role: ui.RoleTypeStaticText,
					},
					ui.Node{
						Name: "OFF",
						Role: ui.RoleTypeToggleButton,
					},
					ui.Node{
						Name: "CheckBox",
						Role: ui.RoleTypeCheckBox,
					},
					ui.Node{
						Name: "seekBar",
						Role: ui.RoleTypeSlider,
					},
					ui.Node{
						Role: ui.RoleTypeSlider,
					},
					ui.Node{
						Name: "ANNOUNCE",
						Role: ui.RoleTypeButton,
					},
					ui.Node{
						Name: "CLICK TO SHOW TOAST",
						Role: ui.RoleTypeButton,
					},
					ui.Node{
						Role: ui.RoleTypeGenericContainer,
					},
				},
			}},
	}
	EditTextActivityTree := ui.Node{
		Name: "Edit Text Activity",
		Role: ui.RoleTypeApplication,
		Children: []ui.Node{
			ui.Node{
				Role: ui.RoleTypeGenericContainer,
				Children: []ui.Node{
					ui.Node{
						Name: "Edit Text Activity",
						Role: ui.RoleTypeStaticText,
					},
					ui.Node{
						Name: "contentDescription",
						Role: ui.RoleTypeTextField,
					},
					ui.Node{
						Name: "hint",
						Role: ui.RoleTypeTextField,
					},
					ui.Node{
						Role:  ui.RoleTypeTextField,
						Value: "text",
					},
				},
			}},
	}

	activities := make(map[string]ui.Node)
	activities[accessibility.MainActivity.Name] = MainActivityTree
	activities[accessibility.EditTextActivity.Name] = EditTextActivityTree
	testActivities := []accessibility.TestActivity{accessibility.MainActivity, accessibility.EditTextActivity}

	testFunc := func(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn, currentActivity accessibility.TestActivity) error {
		actualFileName := axTreeActualTreeFilePrefix + currentActivity.Name + ".json"
		actualFilePath := filepath.Join(s.OutDir(), actualFileName)

		var appRoot *ui.Node
		// Find the root node of Android application.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var err error
			if appRoot, err = ui.Find(ctx, tconn, ui.FindParams{Name: currentActivity.Title, Role: ui.RoleTypeApplication}); err != nil {
				return errors.Wrap(err, "failed to get Android root from accessibility tree")
			}

			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			// When the root could not be found, dump the entire tree.
			if dumpTreeErr := dumpTree(appRoot, actualFilePath); dumpTreeErr != nil {
				return errors.Wrapf(dumpTreeErr, "timed out waiting for appRoot and the previous error is %v", err)
			}
			return errors.Wrapf(err, "timed out waiting for appRoot and wrote the entire tree to %q", actualFileName)
		}

		currentActivityTree := activities[currentActivity.Name]
		if nodeNotFound, err := checkTreeRecursive(ctx, appRoot, currentActivityTree.Children); err != nil {
			diffFileName := axTreeDiffFilePrefix + currentActivity.Name + ".txt"
			diffFilePath := filepath.Join(s.OutDir(), diffFileName)
			if dumpTreeErr := dumpTree(appRoot, actualFilePath); err != nil {
				return errors.Wrapf(dumpTreeErr, "accessibility tree did not match; failed to dump the actual tree, and the previous error is %v", err)
			}
			if dumpTreeErr := dumpTree(&nodeNotFound, diffFilePath); err != nil {
				return errors.Wrapf(dumpTreeErr, "accessibility tree did not match; failed to dump diff node, and the previous error is %v", err)
			}
			return errors.Errorf("accessibility tree did not match (see got:%s, actual:%s)", diffFileName, actualFileName)
		}
		return nil
	}
	accessibility.RunTest(ctx, s, testActivities, testFunc)
}
