// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
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

// matchTree checks actualRoot against expectedRoot, by checking that the root node of actualRoot can be
// found in expectedRoot. This is then matched against the children and performed recursively.
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
		testing.ContextLogf(ctx, "Could not find node: %v, current node: %v", (expectedRoot.findParams()).String(), currNodeStr)
		return false, nil
	}

	actualChildren, err := actualRoot.Children(ctx)
	if err != nil {
		testing.ContextLogf(ctx, "Expected Root: %q", expectedRoot)
		return false, errors.Wrap(err, "failed to get children of current root")
	}
	defer actualChildren.Release(ctx)
	if len(actualChildren) != len(expectedRoot.Children) {
		testing.ContextLogf(ctx, "number of children is incorrect, got %d; want %d", len(actualChildren), len(expectedRoot.Children))
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
						Name: "OFF",
						Role: ui.RoleTypeToggleButton,
					},
					&axTreeNode{
						Name: "CheckBox",
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
	trees[accessibility.MainActivity.Name] = MainActivityTree
	trees[accessibility.EditTextActivity.Name] = EditTextActivityTree
	testActivities := []accessibility.TestActivity{accessibility.MainActivity, accessibility.EditTextActivity}
	testFunc := func(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn, currentActivity accessibility.TestActivity) error {
		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		var appRoot *ui.Node
		var err error
		// Find the root node of Android application.
		if appRoot, err = ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: currentActivity.Title, Role: ui.RoleTypeApplication}, 10*time.Second); err != nil {
			s.Error(err, "failed to get Android root from accessibility tree")
		}
		defer appRoot.Release(ctx)

		if matched, err := matchTree(ctx, appRoot, trees[currentActivity.Name]); err != nil || !matched {
			s.Error(err, "accessibility tree did not match")
		}
		return nil
	}
	accessibility.RunTest(ctx, s, testActivities, testFunc)
}
