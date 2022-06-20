// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mapui

import (
	"encoding/json"
	"fmt"
	"strings"

	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

type node struct {
	Name string    `json:"name"`
	Role role.Role `json:"role"`
}

// nodeFromStringVar should not be called from tests.
// func nodeFromStringVar(rawNode *testing.VarString) func(context.Context) error {
func nodeFromStringVar(rawNode *testing.VarString) *nodewith.Finder {
	finder := nodewith.Empty()

	var node node
	var nodeMap map[string]interface{}

	if err := json.Unmarshal([]byte(rawNode.Value()), &node); err != nil {
		panic(fmt.Sprintf("Failed to unmarshall the node string %q: %v", rawNode.Name(), err))
	}

	if err := json.Unmarshal([]byte(rawNode.Value()), &nodeMap); err != nil {
		panic(fmt.Sprintf("Failed to unmarshall the node string %q: %v", rawNode.Name(), err))
	}

	if _, ok := nodeMap["name"]; ok {
		finder = finder.Name(node.Name)
		delete(nodeMap, "name")
	}

	if _, ok := nodeMap["role"]; ok {
		finder = finder.Role(node.Role)
		delete(nodeMap, "role")
	}

	if len(nodeMap) != 0 {
		keys := make([]string, 0, len(nodeMap))
		for k := range nodeMap {
			keys = append(keys, k)
		}
		panic(fmt.Sprintf("Failed to parse the following field(s): %s", strings.Join(keys, ", ")))
	}

	return finder
}
