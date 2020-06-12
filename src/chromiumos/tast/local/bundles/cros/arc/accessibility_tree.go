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
	axTreeGotTreeFilePrefix = "accessibility_tree_got"
	axTreeWantFilePrefix    = "accessibility_tree_want"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityTree,
		Desc:         "Checks that Chrome accessibility tree for ARC application is correct",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
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

func checkTreeRecursive(ctx context.Context, root *ui.Node, children []ui.Node) error {
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
			testing.ContextLogf(ctx, "Could not find %q", findParams)
			return errors.Wrap(err, "failed to find child")
		} else if childFound == nil {
			return errors.New("found child, but it is null")
		}

		if len(node.Children) > 0 {
			// Want to do this recursively for each child.
			// Find each child in appRoot.
			return checkTreeRecursive(ctx, root, node.Children)
		}
	}
	return nil
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
		gotFileName := axTreeGotTreeFilePrefix + currentActivity.Name + ".json"
		gotFilePath := filepath.Join(s.OutDir(), gotFileName)

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
			if dumpTreeErr := dumpTree(appRoot, gotFilePath); dumpTreeErr != nil {
				return errors.Wrapf(dumpTreeErr, "timed out waiting for appRoot and the previous error is %v", err)
			}
			return errors.Wrapf(err, "timed out waiting for appRoot and wrote the entire tree to %q", gotFileName)
		}

		currentActivityTree := activities[currentActivity.Name]
		if err := checkTreeRecursive(ctx, appRoot, currentActivityTree.Children); err != nil {
			wantFileName := axTreeWantFilePrefix + currentActivity.Name + ".txt"
			wantFilePath := filepath.Join(s.OutDir(), wantFileName)
			if dumpTreeErr := dumpTree(appRoot, gotFilePath); err != nil {
				return errors.Wrapf(dumpTreeErr, "accessibility tree did not match; failed to dump the actual tree, and the previous error is %v", err)
			}
			if dumpTreeErr := dumpTree(&currentActivityTree, wantFilePath); err != nil {
				return errors.Wrapf(dumpTreeErr, "accessibility tree did not match; failed to dump the expected tree, and the previous error is %v", err)
			}
			return errors.Errorf("accessibility tree did not match (see want:%s, got:%s)", wantFileName, gotFileName)
		}
		return nil
	}
	accessibility.RunTest(ctx, s, testActivities, testFunc)
}
