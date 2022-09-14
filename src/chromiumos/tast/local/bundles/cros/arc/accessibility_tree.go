// Copyright 2019 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// axTreeNode represents an accessibility tree.
// ui.FindParams is deliberately not used to avoid nesting,
// and to avoid defining unused properties when we write an expected tree.
type axTreeNode struct {
	Name       string
	Role       role.Role
	Attributes map[string]interface{}
	Children   []*axTreeNode
}

type expectedNode struct {
	CheckBoxAttributes map[string]interface{}
	SeekBarAttributes  map[string]interface{}
}

// findParams constructs a11y.FindParams from the given axTreeNode.
func (n *axTreeNode) findParams() a11y.FindParams {
	return a11y.FindParams{Name: n.Name, Role: n.Role, Attributes: n.Attributes}
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityTree,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Chrome accessibility tree for ARC application is correct",
		Contacts:     []string{"hirokisato@chromium.org", "dtseng@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBootedWithoutUIAutomator",
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Val: expectedNode{
				CheckBoxAttributes: map[string]interface{}{},
				SeekBarAttributes:  map[string]interface{}{},
			},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "vm",
			Val: expectedNode{
				CheckBoxAttributes: map[string]interface{}{"checkedStateDescription": "state description not checked"},
				SeekBarAttributes:  map[string]interface{}{"value": "state description 25"},
			},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// matchTree checks actualRoot against expectedRoot, by checking that the root node of actualRoot can be
// matched to the expectedRoot. This is then matched against the children and performed recursively.
// A boolean is returned, indicating whether or not gotRoot matches wantRoot.
// Error indicates an internal failure, such as connecting to Chrome or invoking the JavaScript.
func matchTree(ctx context.Context, actualRoot *a11y.Node, expectedRoot *axTreeNode) (bool, error) {
	// Check the root node.
	if found, err := actualRoot.Matches(ctx, expectedRoot.findParams()); err != nil {
		return false, err
	} else if !found {
		currNodeStr, err := actualRoot.ToString(ctx)
		if err != nil {
			return false, err
		}
		testing.ContextLogf(ctx, "Node did not match, got %+v; want %v", currNodeStr, expectedRoot.findParams())
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
		Name: arca11y.MainActivity.Title,
		Role: role.Application,
		Children: []*axTreeNode{
			{
				Role: role.GenericContainer,
				Children: []*axTreeNode{
					{
						Name: "Main Activity",
						Role: role.StaticText,
					},
					{
						Name:       "OFF",
						Role:       role.ToggleButton,
						Attributes: map[string]interface{}{"tooltip": "button tooltip"},
					},
					{
						Name:       "CheckBox",
						Role:       role.CheckBox,
						Attributes: map[string]interface{}{"tooltip": "checkbox tooltip"},
					},
					{
						Name:       "CheckBoxWithStateDescription",
						Role:       role.CheckBox,
						Attributes: s.Param().(expectedNode).CheckBoxAttributes,
					},
					{
						Name:       "seekBar",
						Role:       role.Slider,
						Attributes: s.Param().(expectedNode).SeekBarAttributes,
					},
					{
						Role: role.Slider,
					},
					{
						Name: "ANNOUNCE",
						Role: role.Button,
					},
					{
						Name: "CLICK TO SHOW TOAST",
						Role: role.Button,
					},
					{
						Role: role.GenericContainer,
					},
				},
			},
		},
	}

	EditTextActivityTree := &axTreeNode{
		Name: arca11y.EditTextActivity.Title,
		Role: role.Application,
		Children: []*axTreeNode{
			{
				Role: role.GenericContainer,
				Children: []*axTreeNode{
					{
						Name: "Edit Text Activity",
						Role: role.StaticText,
					},
					{
						Name: "contentDescription",
						Role: role.TextField,
					},
					{
						Name: "hint",
						Role: role.TextField,
					},
					{
						Role:       role.TextField,
						Attributes: map[string]interface{}{"value": "text"},
					},
				},
			},
		},
	}

	LiveRegionActivityTree := &axTreeNode{
		Name: "Live Region Activity",
		Role: role.Application,
		Children: []*axTreeNode{
			{
				Role: role.GenericContainer,
				Children: []*axTreeNode{
					{
						Name: "Live Region Activity",
						Role: role.StaticText,
					},
					{
						Name: "CHANGE POLITE LIVE REGION",
						Role: role.Button,
					},
					{
						Name: "CHANGE ASSERTIVE LIVE REGION",
						Role: role.Button,
					},
					{
						Name: "Initial text",
						Role: role.StaticText,
						Attributes: map[string]interface{}{
							"containerLiveStatus": "polite",
							"liveStatus":          "polite",
						},
					},
					{
						Name: "Initial text",
						Role: role.StaticText,
						Attributes: map[string]interface{}{
							"containerLiveStatus": "assertive",
							"liveStatus":          "assertive",
						},
					},
				},
			},
		},
	}

	trees := map[arca11y.TestActivity]*axTreeNode{
		arca11y.MainActivity:       MainActivityTree,
		arca11y.EditTextActivity:   EditTextActivityTree,
		arca11y.LiveRegionActivity: LiveRegionActivityTree,
	}

	testActivities := []arca11y.TestActivity{
		arca11y.MainActivity, arca11y.EditTextActivity, arca11y.LiveRegionActivity,
	}

	testFunc := func(ctx context.Context, cvconn *a11y.ChromeVoxConn, tconn *chrome.TestConn, currentActivity arca11y.TestActivity) error {
		expectedTree := trees[currentActivity]
		var appRoot *a11y.Node
		var err error
		// Find the root node of Android application.
		if appRoot, err = a11y.FindWithTimeout(ctx, tconn, expectedTree.findParams(), 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to get Android root from accessibility tree")
		}
		defer appRoot.Release(ctx)

		if matched, err := matchTree(ctx, appRoot, expectedTree); err != nil || !matched {
			return errors.Wrap(err, "accessibility tree did not match")
		}
		return nil
	}
	arca11y.RunTest(ctx, s, testActivities, testFunc)
}
