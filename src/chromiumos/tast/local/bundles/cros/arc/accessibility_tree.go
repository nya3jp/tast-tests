// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
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
	Name            string
	Role            role.Role
	Attributes      map[string]interface{}
	Children        []*axTreeNode
	StandardActions []string
}

type expectedNode struct {
	CheckBoxAttributes map[string]interface{}
	SeekBarAttributes  map[string]interface{}
}

// matches wraps the match of AutomationNode and adds checks for list attributes, because
// match function of AutomationNode in javascript doesn't support the containment condition
// of list attributes.
func (n axTreeNode) matches(ctx context.Context, actual *a11y.Node) (bool, error) {
	if n.StandardActions != nil {
		expected := n.StandardActions
		actual, err := actual.StandardActions(ctx)
		if err != nil {
			return false, errors.Wrap(err, "failed to get actual standard actions")
		}
		if !hasAllStandardActions(ctx, expected, actual) {
			testing.ContextLogf(
				ctx,
				"standard actions didn't match. expected: %s, got: %s",
				expected,
				actual)
			return false, nil
		}
	}

	return actual.Matches(ctx, n.findParams())
}

func hasAllStandardActions(ctx context.Context, expected, actual []string) bool {
	for _, action := range expected {
		found := false
		for _, actualAction := range actual {
			if action == actualAction {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
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
	if found, err := expectedRoot.matches(ctx, actualRoot); err != nil {
		return false, err
	} else if !found {
		currNodeStr, err := actualRoot.ToString(ctx)
		if err != nil {
			return false, err
		}
		testing.ContextLogf(ctx, "Node did not match, got %+v; want %+v", currNodeStr, expectedRoot)
		return false, nil
	}

	actualChildren, err := actualRoot.Children(ctx)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to get children of %+v", expectedRoot)
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
						Role: role.Button,
						Attributes: map[string]interface{}{
							"name": regexp.MustCompile(`(ANNOUNCE|Announce)`),
						},
					},
					{
						Role: role.Button,
						Attributes: map[string]interface{}{
							"name": regexp.MustCompile(`(CLICK TO SHOW TOAST|Click to show toast)`),
						},
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
						Role: role.Button,
						Attributes: map[string]interface{}{
							"name": regexp.MustCompile(
								`(CHANGE POLITE LIVE REGION|Change Polite Live Region)`,
							),
						},
					},
					{
						Role: role.Button,
						Attributes: map[string]interface{}{
							"name": regexp.MustCompile(
								`(CHANGE ASSERTIVE LIVE REGION|Change Assertive Live Region)`,
							),
						},
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

	ActionActivityTree := &axTreeNode{
		Name: "Action Activity",
		Role: role.Application,
		Children: []*axTreeNode{
			{
				Role: role.GenericContainer,
				Children: []*axTreeNode{
					{
						Name: "Action Activity",
						Role: role.StaticText,
					},
					{
						Role: role.Button,
						Attributes: map[string]interface{}{
							"name": regexp.MustCompile(
								`(LONG CLICK|Long Click)`,
							),
						},
						StandardActions: []string{
							"longClick",
						},
					},
					{
						Role: role.Button,
						Attributes: map[string]interface{}{
							"name": regexp.MustCompile(
								`(LABEL|Label)`,
							),
							"doDefaultLabel": "perform click",
							"longClickLabel": "perform long click",
						},
						StandardActions: []string{
							"longClick",
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
		arca11y.ActionActivity:     ActionActivityTree,
	}

	testActivities := []arca11y.TestActivity{
		arca11y.MainActivity, arca11y.EditTextActivity, arca11y.LiveRegionActivity, arca11y.ActionActivity,
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
