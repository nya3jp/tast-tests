// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package useractions

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

// UserContext represents the user context in the test.
type UserContext struct {
	testName          string
	cr                *chrome.Chrome
	tconn             *chrome.TestConn
	outputDir         string
	contextAttributes map[string]string
	contextTags       map[string]bool
}

// UserAction represents the user action.
type UserAction struct {
	name             string
	action           action.Action
	userContext      *UserContext
	actionAttributes map[string]string
	actionTags       map[string]bool
	ifSuccessFunc    func(ctx context.Context) error
	ifFailFunc       func(ctx context.Context) error
	finalFunc        func(ctx context.Context) error
}

// NewUserContext returns a new user context.
func NewUserContext(testName string, cr *chrome.Chrome, tconn *chrome.TestConn, outputDir string, contextAttributes map[string]string, contextTags []string) *UserContext {
	contextTagsMap := make(map[string]bool)
	for _, tag := range contextTags {
		contextTagsMap[tag] = true
	}

	return &UserContext{
		testName,
		cr,
		tconn,
		outputDir,
		contextAttributes,
		contextTagsMap,
	}
}

// NewUserAction returns a new user action.
func NewUserAction(name string, action action.Action, uc *UserContext, uac UserActionCfg) *UserAction {
	return fromAction(name, action, uc, &uac)
}

// SetAttribute set the value of an attribute of the user context.
func (uc *UserContext) SetAttribute(name, value string) *UserContext {
	uc.contextAttributes[name] = value
	return uc
}

// RemoveAttribute removes an attribute of the user context.
func (uc *UserContext) RemoveAttribute(name string) *UserContext {
	delete(uc.contextAttributes, name)
	return uc
}

// Attributes returns all attributes of the user context.
func (uc *UserContext) Attributes() map[string]string {
	return uc.contextAttributes
}

// AddTags adds tags to the user context.
func (uc *UserContext) AddTags(actionTags []string) *UserContext {
	for _, newTag := range actionTags {
		uc.contextTags[newTag] = true
	}
	return uc
}

// RemoveTags removes tags from the user context.
func (uc *UserContext) RemoveTags(actionTags []string) *UserContext {
	for _, tag := range actionTags {
		delete(uc.contextTags, tag)
	}
	return uc
}

// Chrome returns the Chrome instance from the user context.
func (uc *UserContext) Chrome() *chrome.Chrome {
	return uc.cr
}

// TestAPIConn returns the test connection from the user context.
func (uc *UserContext) TestAPIConn() *chrome.TestConn {
	return uc.tconn
}

// UserActionCfg represents optional configurations of an user action.
type UserActionCfg struct {
	ActionAttributes map[string]string
	ActionTags       []string
	IfSuccessFunc    func(ctx context.Context) error
	IfFailFunc       func(ctx context.Context) error
	FinalFunc        func(ctx context.Context) error
}

func fromAction(name string, action action.Action, uc *UserContext, uac *UserActionCfg) *UserAction {
	if uac == nil {
		uac = &UserActionCfg{}
	}

	actionTagsMap := make(map[string]bool)
	for _, tag := range uac.ActionTags {
		actionTagsMap[tag] = true
	}

	return &UserAction{
		name:             name,
		action:           action,
		userContext:      uc,
		actionAttributes: uac.ActionAttributes,
		actionTags:       actionTagsMap,
		ifSuccessFunc:    uac.IfSuccessFunc,
		ifFailFunc:       uac.IfFailFunc,
		finalFunc:        uac.FinalFunc,
	}
}

// RunAction runs a action.Action as an user action and records detailed running information.
func (uc *UserContext) RunAction(ctx context.Context, name string, action action.Action, uac *UserActionCfg) error {
	userAction := fromAction(name, action, uc, uac)
	return userAction.Run(ctx)
}

// RunActionAsSubTest runs a action.Action as an user action in sub test and records detailed running information.
func (uc *UserContext) RunActionAsSubTest(ctx context.Context, s *testing.State, name string, action action.Action, uac *UserActionCfg, terminateOnError bool) { // NOLINT
	userAction := fromAction(name, action, uc, uac)
	userAction.RunAsSubTest(ctx, s, terminateOnError)
}

// RunAsSubTest runs an user action in sub test and records detailed running information.
func (ua *UserAction) RunAsSubTest(ctx context.Context, s *testing.State, terminateOnError bool) { // NOLINT
	testPass := s.Run(ctx, ua.name, func(ctx context.Context, s *testing.State) { // NOLINT
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, filepath.Join(s.OutDir(), ua.name), s.HasError, ua.userContext.cr, "ui_tree_"+ua.name)
		if err := ua.Run(ctx); err != nil {
			s.Fatalf("Failed to run user action %q: %v", ua.name, err)
		}
	})

	if !testPass && terminateOnError {
		s.Fatalf("Terminate test case due to failed action %q", ua.name)
	}
}

// Run runs an user action and records detailed running information.
func (ua *UserAction) Run(ctx context.Context) (err error) {
	// Combine context attributes and action attributes.
	// Action attributes will replace context attributes if the same name.
	combinedAttributes := make(map[string]string)
	for k, v := range ua.userContext.contextAttributes {
		combinedAttributes[k] = v
	}
	for k, v := range ua.actionAttributes {
		combinedAttributes[k] = v
	}

	// Combine context tags and action tags.
	// It should be tagged if either one equals True.
	combinedTagsMap := make(map[string]bool)
	for k, v := range ua.userContext.contextTags {
		if v {
			combinedTagsMap[k] = true
		}
	}
	for k, v := range ua.actionTags {
		if v {
			combinedTagsMap[k] = true
		}
	}

	combinedTags := make([]string, 0, len(combinedTagsMap))
	for k := range combinedTagsMap {
		combinedTags = append(combinedTags, k)
	}

	startTime := time.Now()
	defer func(ctx context.Context) {
		endTime := time.Now()
		result := &actionResult{
			actionName: ua.name,
			testName:   ua.userContext.testName,
			duration:   endTime.Sub(startTime),
			attributes: combinedAttributes,
			tags:       combinedTags,
			pass:       err == nil,
			err:        err,
		}
		if err := result.writeToFile(ctx, ua.userContext.outputDir); err != nil {
			testing.ContextLog(ctx, "Warning: Failed to write user action result: ", err)
		}
	}(ctx)
	err = ua.action(ctx)
	if err == nil && ua.ifSuccessFunc != nil {
		if successFuncError := ua.ifSuccessFunc(ctx); successFuncError != nil {
			testing.ContextLogf(ctx, "IfSuccessFunc failed in action %q: %v", ua.name, successFuncError)
		}
	}
	if err != nil && ua.ifFailFunc != nil {
		if failFuncError := ua.ifFailFunc(ctx); failFuncError != nil {
			testing.ContextLogf(ctx, "IfFailFunc failed in action %q: %v", ua.name, failFuncError)
		}
	}
	if ua.finalFunc != nil {
		if finalFuncError := ua.finalFunc(ctx); finalFuncError != nil {
			testing.ContextLogf(ctx, "IfFailFunc failed in action %q: %v", ua.name, finalFuncError)
		}
	}
	return err
}

type actionResult struct {
	actionName string
	testName   string
	attributes map[string]string
	tags       []string
	duration   time.Duration
	pass       bool
	err        error
}

// writeToFile writes action result into "actionLogFileName" in the outDir.
func (ar *actionResult) writeToFile(ctx context.Context, outDir string) error {
	const actionLogFileName = "user_action_log.csv"
	filePath := filepath.Join(outDir, actionLogFileName)
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to open file at %q", filePath)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	attrStr, err := json.Marshal(ar.attributes)
	if err != nil {
		return errors.Wrapf(err, "failed to JSON encoding user action attributes: %v", ar.attributes)
	}

	errMessage := ""
	if err != nil {
		errMessage = fmt.Sprintf("%v", ar.err)
	}
	result := []string{ar.actionName, ar.testName, fmt.Sprintf("%s", attrStr), fmt.Sprintf("%s", strings.Join(ar.tags, ", ")), fmt.Sprintf("%d", int64(ar.duration/time.Millisecond)), fmt.Sprintf("%s", strconv.FormatBool(ar.pass)), fmt.Sprintf("%s", errMessage)}
	return w.Write(result)
}
