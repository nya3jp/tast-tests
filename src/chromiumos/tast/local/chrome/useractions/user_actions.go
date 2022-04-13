// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package useractions contains the definition of UserContext and UserAction.
// It also provides helper functions to use in test cases.
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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// UserContext represents the user context in the test.
type UserContext struct {
	testName   string
	cr         *chrome.Chrome
	tconn      *chrome.TestConn
	outputDir  string
	attributes map[string]string
	tags       map[ActionTag]struct{}
}

// UserActionCfg represents optional configurations of a user action.
type UserActionCfg struct {
	Attributes     map[string]string
	Tags           []ActionTag
	ValidateResult action.Action                                    // validateResult should only check the outcome of the user action.
	Callback       func(ctx context.Context, actionErr error) error // callback takes action error as input.
}

// UserAction represents the user action.
type UserAction struct {
	name          string
	action        action.Action
	userActionCfg *UserActionCfg
	userContext   *UserContext
}

// NewUserContext returns a new user context.
func NewUserContext(testName string, cr *chrome.Chrome, tconn *chrome.TestConn, outputDir string, attributes map[string]string, tags []ActionTag) *UserContext {
	tagsMap := make(map[ActionTag]struct{})
	if tags != nil {
		for _, tag := range tags {
			tagsMap[tag] = struct{}{}
		}
	}

	if attributes == nil {
		attributes = make(map[string]string)
	}

	return &UserContext{
		testName,
		cr,
		tconn,
		outputDir,
		attributes,
		tagsMap,
	}
}

// NewUserAction returns a new user action.
func NewUserAction(name string, action action.Action, uc *UserContext, uac *UserActionCfg) *UserAction {
	return fromAction(name, action, uc, uac)
}

// NewUserAction returns a new user action.
func (uc *UserContext) NewUserAction(name string, action action.Action, uac *UserActionCfg) *UserAction {
	return fromAction(name, action, uc, uac)
}

// InvalidUserAction creates a user action to return error directly.
func (uc *UserContext) InvalidUserAction(err error) *UserAction {
	return fromAction("Invalid action", func(ctx context.Context) error {
		return err
	}, uc, nil)
}

// SetTestName sets the test name of the user context.
func (uc *UserContext) SetTestName(testName string) {
	uc.testName = testName
}

// SetAttribute set the value of an attribute of the user context.
func (uc *UserContext) SetAttribute(name, value string) {
	uc.attributes[name] = value
}

// RemoveAttribute removes an attribute of the user context.
func (uc *UserContext) RemoveAttribute(name string) {
	delete(uc.attributes, name)
}

// Attributes returns all attributes of the user context.
func (uc *UserContext) Attributes() map[string]string {
	return uc.attributes
}

// AddTags adds tags to the user context.
func (uc *UserContext) AddTags(actionTags []ActionTag) {
	for _, newTag := range actionTags {
		uc.tags[newTag] = struct{}{}
	}
}

// RemoveTags removes tags from the user context.
func (uc *UserContext) RemoveTags(actionTags []ActionTag) {
	for _, tag := range actionTags {
		delete(uc.tags, tag)
	}
}

// Chrome returns the Chrome instance from the user context.
func (uc *UserContext) Chrome() *chrome.Chrome {
	return uc.cr
}

// TestAPIConn returns the test connection from the user context.
func (uc *UserContext) TestAPIConn() *chrome.TestConn {
	return uc.tconn
}

func fromAction(name string, action action.Action, uc *UserContext, uac *UserActionCfg) *UserAction {
	if uac == nil {
		uac = &UserActionCfg{}
	}

	if uac.Attributes == nil {
		uac.Attributes = make(map[string]string)
	}

	if uac.Tags == nil {
		uac.Tags = []ActionTag{}
	}

	return &UserAction{
		name:          name,
		action:        action,
		userContext:   uc,
		userActionCfg: uac,
	}
}

// RunAction runs a action.Action as a user action and records detailed running information.
func (uc *UserContext) RunAction(ctx context.Context, name string, action action.Action, uac *UserActionCfg) error {
	userAction := fromAction(name, action, uc, uac)
	return userAction.Run(ctx)
}

// Run runs a user action and records detailed running information.
func (ua *UserAction) Run(ctx context.Context) (err error) {
	// Combine context attributes and action attributes.
	// Action attributes will replace context attributes if the same name.
	combinedAttributes := make(map[string]string)
	for k, v := range ua.userContext.attributes {
		combinedAttributes[k] = v
	}
	for k, v := range ua.userActionCfg.Attributes {
		combinedAttributes[k] = v
	}

	// Combine context tags and action tags.
	// It should be tagged if either one equals True.
	combinedTagsMap := make(map[ActionTag]struct{})
	for k := range ua.userContext.tags {
		combinedTagsMap[k] = struct{}{}
	}

	for _, tag := range ua.userActionCfg.Tags {
		combinedTagsMap[tag] = struct{}{}
	}

	combinedTags := make([]ActionTag, 0, len(combinedTagsMap))
	for k := range combinedTagsMap {
		combinedTags = append(combinedTags, k)
	}

	startTime := time.Now()
	defer func(ctx context.Context) {
		endTime := time.Now()
		result := &actionResult{
			actionName: ua.name,
			testName:   ua.userContext.testName,
			startTime:  startTime,
			endTime:    endTime,
			attributes: combinedAttributes,
			tags:       combinedTags,
			pass:       err == nil,
			err:        err,
		}
		if err := result.writeToFile(ua.userContext.outputDir); err != nil {
			testing.ContextLog(ctx, "Warning: Failed to write user action result: ", err)
		}
	}(ctx)
	err = ua.action(ctx)
	// Only validate action result if the action finished without error.
	if err == nil && ua.userActionCfg.ValidateResult != nil {
		err = ua.userActionCfg.ValidateResult(ctx)
	}

	if ua.userActionCfg.Callback != nil {
		if callbackError := ua.userActionCfg.Callback(ctx, err); callbackError != nil {
			testing.ContextLogf(ctx, "callback failed in action %q: %v", ua.name, callbackError)
		}
	}
	return err
}

// Name returns the name of the user action.
func (ua *UserAction) Name() string {
	return ua.name
}

// UserContext returns the user context instance of the user action.
func (ua *UserAction) UserContext() *UserContext {
	return ua.userContext
}

// SetAttribute set the value of an attribute of the user action.
func (ua *UserAction) SetAttribute(key, value string) {
	ua.userActionCfg.Attributes[key] = value
}

// AddTags adds tags to the user action.
func (ua *UserAction) AddTags(actionTags []ActionTag) {
	ua.userActionCfg.Tags = append(ua.userActionCfg.Tags, actionTags...)
}

type actionResult struct {
	actionName string
	testName   string
	attributes map[string]string
	tags       []ActionTag
	startTime  time.Time
	endTime    time.Time
	pass       bool
	err        error
}

const actionTimeFormat = "2006-01-02 15:04:05.000"

func (ar *actionResult) stringArray() ([]string, error) {
	attrStr, err := json.Marshal(ar.attributes)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to JSON encoding user action attributes: %v", ar.attributes)
	}

	errMessage := ""
	if ar.err != nil {
		errMessage = fmt.Sprintf("%v", ar.err)
	}

	var tags []string
	for _, tag := range ar.tags {
		tags = append(tags, string(tag))
	}

	return []string{
		ar.actionName,
		ar.testName,
		fmt.Sprintf("%s", attrStr),
		fmt.Sprintf("%s", strings.Join(tags, ", ")),
		fmt.Sprintf("%s", ar.startTime.Format(actionTimeFormat)),
		fmt.Sprintf("%s", ar.endTime.Format(actionTimeFormat)),
		fmt.Sprintf("%s", strconv.FormatBool(ar.pass)),
		fmt.Sprintf("%s", errMessage),
	}, nil
}

// writeToFile writes action result into "actionLogFileName" in the outDir.
func (ar *actionResult) writeToFile(outDir string) error {
	strArray, err := ar.stringArray()
	if err != nil {
		return err
	}

	const actionLogFileName = "user_action_log.csv"
	filePath := filepath.Join(outDir, actionLogFileName)
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to open file at %q", filePath)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	return w.Write(strArray)
}
