// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
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

const axTreeGotTreeFilePrefix = "accessibility_tree_got"

type arcAXNode struct {
	Name       string
	Role       ui.RoleType
	Attributes map[string]interface{}
	Children   []*arcAXNode
}

func (n *arcAXNode) FindParam() ui.FindParams {
	return ui.FindParams{Name: n.Name, Role: n.Role, Attributes: n.Attributes}
}

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

// dumpTree writes the given accessibility tree to the file specified by filepath.
func dumpTree(ctx context.Context, tree *ui.Node, filepath string) error {
	treeString, err := tree.ToString(ctx)
	if err != nil {
		return err
	}

	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(treeString)
	if err != nil {
		return err
	}

	return nil
}

// matchTree checks gotRoot against actualRoot, by checking that the root node of gotRoot can be
// found in actualRoot. This is then matched against the children and performed recursively.
// A boolean is returned, indicating whether or not gotRoot matches actualRoot.
// Error indicates an internal failure, such as connecting to Chrome or invoking the JavaScript.
func matchTree(ctx context.Context, gotRoot *ui.Node, actualRoot *arcAXNode) (bool, error) {
	// Check the root node.
	if found, err := gotRoot.Matches(ctx, actualRoot.FindParam()); err != nil {
		return false, err
	} else if !found {
		testing.ContextLogf(ctx, "Could not find node %q", actualRoot.FindParam())
		return false, nil
	}

	gotChildren, err := gotRoot.Children(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get children of current root")
	}
	if len(gotChildren) != len(actualRoot.Children) {
		testing.ContextLogf(ctx, "number of children is incorrect, got %d; want %d", len(gotChildren), len(actualRoot.Children))
		return false, nil
	}

	for i, child := range actualRoot.Children {
		if found, err := matchTree(ctx, gotChildren[i], child); err != nil {
			return false, err
		} else if !found {
			return false, nil
		}
	}
	return true, nil
}

func AccessibilityTree(ctx context.Context, s *testing.State) {
	MainActivityTree := &arcAXNode{
		Name: "Main Activity",
		Role: ui.RoleTypeApplication,
		Children: []*arcAXNode{
			&arcAXNode{
				Role: ui.RoleTypeGenericContainer,
				Children: []*arcAXNode{
					&arcAXNode{
						Name: "Main Activity",
						Role: ui.RoleTypeStaticText,
					},
					&arcAXNode{
						Name: "OFF",
						Role: ui.RoleTypeToggleButton,
					},
					&arcAXNode{
						Name: "CheckBox",
						Role: ui.RoleTypeCheckBox,
					},
					&arcAXNode{
						Name: "seekBar",
						Role: ui.RoleTypeSlider,
					},
					&arcAXNode{
						Role: ui.RoleTypeSlider,
					},
					&arcAXNode{
						Name: "ANNOUNCE",
						Role: ui.RoleTypeButton,
					},
					&arcAXNode{
						Name: "CLICK TO SHOW TOAST",
						Role: ui.RoleTypeButton,
					},
					&arcAXNode{
						Role: ui.RoleTypeGenericContainer,
					},
				},
			},
		},
	}
	EditTextActivityTree := &arcAXNode{
		Name: "Edit Text Activity",
		Role: ui.RoleTypeApplication,
		Children: []*arcAXNode{
			&arcAXNode{
				Role: ui.RoleTypeGenericContainer,
				Children: []*arcAXNode{
					&arcAXNode{
						Name: "Edit Text Activity",
						Role: ui.RoleTypeStaticText,
					},
					&arcAXNode{
						Name: "contentDescription",
						Role: ui.RoleTypeTextField,
					},
					&arcAXNode{
						Name: "hint",
						Role: ui.RoleTypeTextField,
					},
					&arcAXNode{
						Role:       ui.RoleTypeTextField,
						Attributes: map[string]interface{}{"value": "text"},
					},
				},
			},
		},
	}

	trees := make(map[string]*arcAXNode)
	trees[accessibility.MainActivity.Name] = MainActivityTree
	trees[accessibility.EditTextActivity.Name] = EditTextActivityTree
	testActivities := []accessibility.TestActivity{accessibility.MainActivity, accessibility.EditTextActivity}

	testFunc := func(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn, currentActivity accessibility.TestActivity) error {
		gotFileName := axTreeGotTreeFilePrefix + currentActivity.Name + ".txt"
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
			if dumpTreeErr := dumpTree(ctx, appRoot, gotFilePath); dumpTreeErr != nil {
				s.Error(dumpTreeErr)
				return errors.Wrapf(err, "failed to write tree to %q", gotFileName)
			}

			return errors.Wrap(err, "timed out waiting for appRoot")
		}

		currentActivityTree := trees[currentActivity.Name]

		matchFound, err := matchTree(ctx, appRoot, currentActivityTree)
		if err != nil || !matchFound {
			nestedErr := func() error {
				if err := dumpTree(ctx, appRoot, gotFilePath); err != nil {
					return errors.Wrap(err, "failed to dump the actual tree")
				}
				return errors.Errorf("see want: %s", gotFileName)
			}()
			if nestedErr != nil {
				s.Error(nestedErr)
			}
			return errors.Wrap(err, "accessibility tree did not match")
		}
		return nil
	}
	accessibility.RunTest(ctx, s, testActivities, testFunc)
}
