// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ActionReplay documentation
type ActionReplay struct {
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
type ActionFunc func(ctx *ActionReplay, json interface{})

func (ctx *ActionReplay) actionMaximize(json interface{}) {
	testing.ContextLogf(ctx.ctx, "maximize: (%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionMinimize(json interface{}) {
	testing.ContextLogf(ctx.ctx, "minimize: (%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionCrosShell(json interface{}) {
	testing.ContextLogf(ctx.ctx, "cros shell:(%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionAdbShell(json interface{}) {
	testing.ContextLogf(ctx.ctx, "adb shell: (%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionUIAutomator(json interface{}) {
	testing.ContextLogf(ctx.ctx, "ui automator: (%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionBroadcast(json interface{}) {
	testing.ContextLogf(ctx.ctx, "broadcast: (%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionScreenshot(json interface{}) {
	testing.ContextLogf(ctx.ctx, "screnshot: (%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionInstallAPK(json interface{}) {
	testing.ContextLogf(ctx.ctx, "install: (%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionStartActivity(json interface{}) {
	testing.ContextLogf(ctx.ctx, "start: (%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionClamshellMode(json interface{}) {
	testing.ContextLog(ctx.ctx, "clamshell: (%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionTabletMode(json interface{}) {
	testing.ContextLogf(ctx.ctx, "tablet: (%v, %T)\n", json, json)
}

func (ctx *ActionReplay) actionKeyboard(json interface{}) {
	testing.ContextLogf(ctx.ctx, "keyboard: (%v, %T)\n", json, json)

	d := json.(map[string]interface{})
	value := d["accel"].(string)

	testing.ContextLogf(ctx.ctx, "value is: '%s'\n", value)

	ew, err := input.Keyboard(ctx.ctx)
	if err != nil {
		testing.ContextLogf(ctx.ctx, "Failed to open keyboard device: %T", err)
	}
	defer ew.Close()

	// ew.Accel(ctx.ctx, value)
	// ew.Event(input.EV_KEY, input.KEY_BURGER, 1)
	// ew.Sync()
	// ew.Event(input.EV_KEY, input.KEY_BURGER, 0)
	// ew.Sync()
	ew.Accel(ctx.ctx, "Search+l")
}

// Init documentation
func (ctx *ActionReplay) Init(origCtx context.Context, filename string) error {
	ctx.ctx = origCtx

	testing.ContextLogf(ctx.ctx, "Trying to open: %s\n", filename)

	// Open our jsonFile
	jsonFile, err := os.Open(filename)
	// if we os.Open returns an error then handle it
	if err != nil {
		testing.ContextLog(ctx.ctx, err)
		return err
	}
	testing.ContextLogf(ctx.ctx, "Successfully Opened file: %s\n", filename)
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var testEntry TestEntry

	json.Unmarshal([]byte(byteValue), &testEntry)

	ctx.TestEntry = testEntry
	ctx.Actions = map[string]ActionFunc{
		"adb_shell":      (*ActionReplay).actionAdbShell,
		"broadcast":      (*ActionReplay).actionBroadcast,
		"cros_shell":     (*ActionReplay).actionCrosShell,
		"install_apk":    (*ActionReplay).actionInstallAPK,
		"maximize":       (*ActionReplay).actionMaximize,
		"minimize":       (*ActionReplay).actionMinimize,
		"start_activity": (*ActionReplay).actionStartActivity,
		"uiautomator":    (*ActionReplay).actionUIAutomator,
		"screenshot":     (*ActionReplay).actionScreenshot,
		"clamshell_mode": (*ActionReplay).actionClamshellMode,
		"tablet_mode":    (*ActionReplay).actionTabletMode,
		"keyboard":       (*ActionReplay).actionKeyboard,
	}

	return nil
}

func (ctx *ActionReplay) runAction(actionName string, actionArgs interface{}) {
	if actionFn, ok := ctx.Actions[actionName]; ok {
		actionFn(ctx, actionArgs)
	} else {
		testing.ContextLogf(ctx.ctx, "Unknown action name: %s\n", actionName)
	}
}

// Setup documentation
func (ctx *ActionReplay) Setup() {
	testing.ContextLog(ctx.ctx, "Setup")
	for actionName, actionArgs := range ctx.TestEntry.Setup {
		ctx.runAction(actionName, actionArgs)
	}
}

// Run documentation
func (ctx *ActionReplay) Run() {
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
func (ctx *ActionReplay) Cleanup() {
	testing.ContextLog(ctx.ctx, "Cleanup")
}
