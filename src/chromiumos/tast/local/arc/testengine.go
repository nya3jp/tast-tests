// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"chromiumos/tast/testing"
)

// TestContext documentation
type TestContext struct {
	Filename   string // JSON filename
	TestEntry  TestEntry
	Actions    map[string]ActionFunc // map with all the action
	LastResult int                   // af
	ctx        context.Context
}

// TestEntry documentation
// JSON file could have these 4 elements
//	"setup": {}
//  "tests": []
//  "cleanup": {}
//  "metadata": {}
type TestEntry struct {
	Setup    map[string]interface{}
	Tests    []SingleTest
	Cleanup  map[string]interface{}
	Metadata map[string]interface{}
}

// SingleTest documentation
type SingleTest struct {
	Name     string
	Actions  []interface{}
	Expected map[string]interface{}
}

// ActionFunc documentation
type ActionFunc func(ctx *TestContext, json interface{})

func (ctx *TestContext) actionMaximize(json interface{}) {
	testing.ContextLogf(ctx.ctx, "maximize: (%v, %T)\n", json, json)
}

func (ctx *TestContext) actionMinimize(json interface{}) {
	testing.ContextLogf(ctx.ctx, "minimize: (%v, %T)\n", json, json)
}

func (ctx *TestContext) actionCrosShell(json interface{}) {
	testing.ContextLogf(ctx.ctx, "cros shell:(%v, %T)\n", json, json)
}

func (ctx *TestContext) actionAdbShell(json interface{}) {
	testing.ContextLogf(ctx.ctx, "adb shell: (%v, %T)\n", json, json)
}

func (ctx *TestContext) actionUIAutomator(json interface{}) {
	testing.ContextLogf(ctx.ctx, "ui automator: (%v, %T)\n", json, json)
}

func (ctx *TestContext) actionBroadcast(json interface{}) {
	testing.ContextLogf(ctx.ctx, "broadcast: (%v, %T)\n", json, json)
}

func (ctx *TestContext) actionScreenshot(json interface{}) {
	testing.ContextLogf(ctx.ctx, "screnshot: (%v, %T)\n", json, json)
}

func (ctx *TestContext) actionInstallAPK(json interface{}) {
	testing.ContextLogf(ctx.ctx, "install: (%v, %T)\n", json, json)
}

func (ctx *TestContext) actionStartActivity(json interface{}) {
	testing.ContextLogf(ctx.ctx, "start: (%v, %T)\n", json, json)
}

func (ctx *TestContext) actionClamshellMode(json interface{}) {
	testing.ContextLog(ctx.ctx, "clamshell: (%v, %T)\n", json, json)
}

func (ctx *TestContext) actionTabletMode(json interface{}) {
	testing.ContextLogf(ctx.ctx, "tablet: (%v, %T)\n", json, json)
}

// Init documentation
func (ctx *TestContext) Init(origCtx context.Context, filename string) error {
	ctx.ctx = origCtx

	testing.ContextLogf(ctx.ctx, "Trying to open: %s\n", filename)

	// Open our jsonFile
	jsonFile, err := os.Open(filename)
	// if we os.Open returns an error then handle it
	if err != nil {
		testing.ContextLog(ctx.ctx, err)
		return err
	}
	fmt.Println("Successfully Opened users.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var testEntry TestEntry

	json.Unmarshal([]byte(byteValue), &testEntry)

	ctx.TestEntry = testEntry
	ctx.Actions = map[string]ActionFunc{
		"adb_shell":      (*TestContext).actionAdbShell,
		"broadcast":      (*TestContext).actionBroadcast,
		"cros_shell":     (*TestContext).actionCrosShell,
		"install_apk":    (*TestContext).actionInstallAPK,
		"maximize":       (*TestContext).actionMaximize,
		"minimize":       (*TestContext).actionMinimize,
		"start_activity": (*TestContext).actionStartActivity,
		"uiautomator":    (*TestContext).actionUIAutomator,
		"screenshot":     (*TestContext).actionScreenshot,
		"clamshell_mode": (*TestContext).actionClamshellMode,
		"tablet_mode":    (*TestContext).actionTabletMode,
	}

	return nil
}

func (ctx *TestContext) runAction(actionName string, actionArgs interface{}) {
	if actionFn, ok := ctx.Actions[actionName]; ok {
		actionFn(ctx, actionArgs)
	} else {
		fmt.Printf("Unknown action name: %s\n", actionName)
	}
}

// Setup documentation
func (ctx *TestContext) Setup() {
	testing.ContextLog(ctx.ctx, "Setup")
	for actionName, actionArgs := range ctx.TestEntry.Setup {
		ctx.runAction(actionName, actionArgs)
	}
}

// Run documentation
func (ctx *TestContext) Run() {
	testing.ContextLog(ctx.ctx, "Run")
	for _, testEntry := range ctx.TestEntry.Tests {
		testing.ContextLogf(ctx.ctx, "(%v, %T)\n", testEntry, testEntry)
		testing.ContextLogf(ctx.ctx, "Running test: %s\n", testEntry.Name)
		for _, actionName := range testEntry.Actions {
			switch actionName.(type) {
			case string:
				ctx.runAction(actionName.(string), nil)
			case map[string]interface{}:
				for k, v := range actionName.(map[string]interface{}) {
					ctx.runAction(k, v)
				}
			}
		}
	}
}

// Cleanup documentation
func (ctx *TestContext) Cleanup() {
	testing.ContextLog(ctx.ctx, "Cleanup")
}
