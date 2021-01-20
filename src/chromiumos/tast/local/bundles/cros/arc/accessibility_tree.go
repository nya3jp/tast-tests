// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	arca11y "chromiumos/tast/local/bundles/cros/arc/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// axTreeNode represents an accessibility tree.
// ui.FindParams is deliberately not used to avoid nesting,
// and to avoid defining unused properties when we write an expected tree.
type axTreeNode struct {
	Name       string
	Role       ui.RoleType
	Attributes map[string]interface{}
	Children   []*axTreeNode
}

// findParams constructs ui.FindParams from the given axTreeNode.
func (n *axTreeNode) findParams() ui.FindParams {
	return ui.FindParams{Name: n.Name, Role: n.Role, Attributes: n.Attributes}
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityTree,
		Desc:         "Checks that Chrome accessibility tree for ARC application is correct",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// matchTree checks actualRoot against expectedRoot, by checking that the root node of actualRoot can be
// matched to the expectedRoot. This is then matched against the children and performed recursively.
// A boolean is returned, indicating whether or not gotRoot matches wantRoot.
// Error indicates an internal failure, such as connecting to Chrome or invoking the JavaScript.
func matchTree(ctx context.Context, actualRoot *ui.Node, expectedRoot *axTreeNode) (bool, error) {
	// Check the root node.
	if found, err := actualRoot.Matches(ctx, expectedRoot.findParams()); err != nil {
		return false, err
	} else if !found {
		currNodeStr, err := actualRoot.ToString(ctx)
		if err != nil {
			return false, err
		}
		testing.ContextLogf(ctx, "Node did not match, got %+v; want %v", expectedRoot.findParams(), currNodeStr)
		return false, nil
	}

	actualChildren, err := actualRoot.Children(ctx)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to get children of %+v", expectedRoot.findParams())
		return false, err
	}
	defer actualChildren.Release(ctx)
	if len(actualChildren) != len(expectedRoot.Children) {
		currNodeStr, err := actualRoot.ToString(ctx)
		if err != nil {
			return false, err
		}
		testing.ContextLogf(ctx, "number of children is incorrect, got %d; want %d. Currently at %+v", len(actualChildren), len(expectedRoot.Children), currNodeStr)
		return false, nil
	}

	for i, child := range expectedRoot.Children {
		if found, err := matchTree(ctx, actualChildren[i], child); err != nil {
			return false, err
		} else if !found {
			return false, nil
		}
	}
	return true, nil
}

func AccessibilityTree(ctx context.Context, s *testing.State) {
	MainActivityTree := &axTreeNode{
		Name: "Main Activity",
		Role: ui.RoleTypeApplication,
		Children: []*axTreeNode{
			&axTreeNode{
				Role: ui.RoleTypeGenericContainer,
				Children: []*axTreeNode{
					&axTreeNode{
						Name: "Main Activity",
						Role: ui.RoleTypeStaticText,
					},
					&axTreeNode{
						Name:       "OFF",
						Role:       ui.RoleTypeToggleButton,
						Attributes: map[string]interface{}{"tooltip": "button tooltip"},
					},
					&axTreeNode{
						Name:       "CheckBox",
						Role:       ui.RoleTypeCheckBox,
						Attributes: map[string]interface{}{"tooltip": "checkbox tooltip"},
					},
					&axTreeNode{
						Name: "CheckBoxWithStateDescription",
						Role: ui.RoleTypeCheckBox,
					},
					&axTreeNode{
						Name: "seekBar",
						Role: ui.RoleTypeSlider,
					},
					&axTreeNode{
						Role: ui.RoleTypeSlider,
					},
					&axTreeNode{
						Name: "ANNOUNCE",
						Role: ui.RoleTypeButton,
					},
					&axTreeNode{
						Name: "CLICK TO SHOW TOAST",
						Role: ui.RoleTypeButton,
					},
					&axTreeNode{
						Role: ui.RoleTypeGenericContainer,
					},
				},
			},
		},
	}
	EditTextActivityTree := &axTreeNode{
		Name: "Edit Text Activity",
		Role: ui.RoleTypeApplication,
		Children: []*axTreeNode{
			&axTreeNode{
				Role: ui.RoleTypeGenericContainer,
				Children: []*axTreeNode{
					&axTreeNode{
						Name: "Edit Text Activity",
						Role: ui.RoleTypeStaticText,
					},
					&axTreeNode{
						Name: "contentDescription",
						Role: ui.RoleTypeTextField,
					},
					&axTreeNode{
						Name: "hint",
						Role: ui.RoleTypeTextField,
					},
					&axTreeNode{
						Role:       ui.RoleTypeTextField,
						Attributes: map[string]interface{}{"value": "text"},
					},
				},
			},
		},
	}

	trees := make(map[string]*axTreeNode)
	trees[arca11y.MainActivity.Name] = MainActivityTree
	trees[arca11y.EditTextActivity.Name] = EditTextActivityTree
	testActivities := []arca11y.TestActivity{arca11y.MainActivity, arca11y.EditTextActivity}

	testFunc := func(ctx context.Context, cvconn *a11y.ChromeVoxConn, tconn *chrome.TestConn, currentActivity arca11y.TestActivity) error {
		var appRoot *ui.Node
		var err error
		// Find the root node of Android application.
		if appRoot, err = ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: currentActivity.Title, Role: ui.RoleTypeApplication}, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to get Android root from accessibility tree")
		}
		defer appRoot.Release(ctx)

		if matched, err := matchTree(ctx, appRoot, trees[currentActivity.Name]); err != nil || !matched {
			return errors.Wrap(err, "accessibility tree did not match")
		}
		return nil
	}
	arca11y.RunTest(ctx, s, testActivities, testFunc)
}
