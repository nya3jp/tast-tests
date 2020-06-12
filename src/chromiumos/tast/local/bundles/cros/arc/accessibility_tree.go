// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
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

type arcAXNodeParams struct {
	params   ui.FindParams
	children []arcAXNodeParams
}

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

// dumpTree writes the given accessibility tree to the file specified by filepath.
func dumpTree(ctx context.Context, tree *ui.Node, filepath string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	treeString, err := tree.ToString(ctx)
	if err != nil {
		return err
	}
	_, err = f.WriteString(treeString)
	if err != nil {
		return err
	}

	return nil
}

func dumpOriginalTree(ctx context.Context, tree arcAXNodeParams, filepath string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(fmt.Sprintf("%s", tree))
	if err != nil {
		return err
	}
	return nil
}

func matchTree(ctx context.Context, gotRoot *ui.Node, actualRoot arcAXNodeParams) (bool, error) {
	// First, do node.matches() to check the root node
	if found, err := gotRoot.Matches(ctx, actualRoot.params); err != nil {
		return false, err
	} else if !found {
		return false, errors.Errorf("could not find node %q", actualRoot.params)
	}
	// Maybe add params.toString()?
	gotChildren, err := gotRoot.Children(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get children of current root")
	}
	// Then, check size of children.
	if len(gotChildren) != len(actualRoot.children) {
		return false, errors.Errorf("number of children is incorrect, got %d; want %d", len(gotChildren), len(actualRoot.children))
	}
	// Then iterate for-loop for children and recursively call matchTree()
	for i, child := range actualRoot.children {
		if found, err := matchTree(ctx, gotChildren[i], child); err != nil {
			return false, err
		} else if !found {
			return false, errors.New("did not find node")
		}
	}
	return true, nil
}

func AccessibilityTree(ctx context.Context, s *testing.State) {
	MainActivityTree := arcAXNodeParams{
		params: ui.FindParams{
			Name: "Main Activity",
			Role: ui.RoleTypeApplication,
		},
		children: []arcAXNodeParams{
			arcAXNodeParams{
				params: ui.FindParams{
					Role: ui.RoleTypeGenericContainer,
				},
				children: []arcAXNodeParams{
					arcAXNodeParams{
						params: ui.FindParams{
							Name: "Main Activity",
							Role: ui.RoleTypeStaticText,
						},
					},
					arcAXNodeParams{
						params: ui.FindParams{
							Name: "OFF",
							Role: ui.RoleTypeToggleButton,
						},
					},
					arcAXNodeParams{
						params: ui.FindParams{
							Name: "CheckBox",
							Role: ui.RoleTypeCheckBox,
						},
					},
					arcAXNodeParams{
						params: ui.FindParams{
							Name: "seekBar",
							Role: ui.RoleTypeSlider,
						},
					},
					arcAXNodeParams{
						params: ui.FindParams{
							Role: ui.RoleTypeSlider,
						},
					},
					arcAXNodeParams{
						params: ui.FindParams{
							Name: "ANNOUNCE",
							Role: ui.RoleTypeButton,
						},
					},
					arcAXNodeParams{
						params: ui.FindParams{
							Name: "CLICK TO SHOW TOAST",
							Role: ui.RoleTypeButton,
						},
					},
					arcAXNodeParams{
						params: ui.FindParams{
							Role: ui.RoleTypeGenericContainer,
						},
					},
				},
			},
		},
	}
	EditTextActivityTree := arcAXNodeParams{
		params: ui.FindParams{
			Name: "Edit Text Activity",
			Role: ui.RoleTypeApplication,
		},
		children: []arcAXNodeParams{
			arcAXNodeParams{
				params: ui.FindParams{
					Role: ui.RoleTypeGenericContainer,
				},
				children: []arcAXNodeParams{
					arcAXNodeParams{
						params: ui.FindParams{
							Name: "Edit Text Activity",
							Role: ui.RoleTypeStaticText,
						},
					},
					arcAXNodeParams{
						params: ui.FindParams{
							Name: "contentDescription",
							Role: ui.RoleTypeTextField,
						},
					},
					arcAXNodeParams{
						params: ui.FindParams{
							Name: "hint",
							Role: ui.RoleTypeTextField,
						},
					},
					arcAXNodeParams{
						params: ui.FindParams{
							Role:       ui.RoleTypeTextField,
							Attributes: map[string]interface{}{"value": "text"},
						},
					},
				},
			},
		},
	}

	activities := make(map[string]arcAXNodeParams)
	activities[accessibility.MainActivity.Name] = MainActivityTree
	activities[accessibility.EditTextActivity.Name] = EditTextActivityTree
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
			nestedErr := func() error {
				if err := dumpTree(ctx, appRoot, gotFilePath); err != nil {
					return errors.Wrapf(err, "failed to write tree to %q", gotFileName)
				}
				return nil
			}()

			if nestedErr != nil {
				s.Error("Timed out waiting for appRoot: ", nestedErr)
			}

			return errors.Wrap(err, "timed out waiting for appRoot")
		}

		currentActivityTree := activities[currentActivity.Name]

		matchFound, err := matchTree(ctx, appRoot, currentActivityTree)
		if err != nil || !matchFound {
			nestedErr := func() error {
				wantFileName := axTreeWantFilePrefix + currentActivity.Name + ".txt"
				wantFilePath := filepath.Join(s.OutDir(), wantFileName)
				if err := dumpTree(ctx, appRoot, gotFilePath); err != nil {
					return errors.Wrap(err, "failed to dump the actual tree")
				}
				if err := dumpOriginalTree(ctx, currentActivityTree, wantFilePath); err != nil {
					return errors.Wrap(err, "failed to dump the expected tree")
				}
				return errors.Errorf("(see want:%s, got:%s)", wantFileName, gotFileName)
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
