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
	testName          string
	cr                *chrome.Chrome
	tconn             *chrome.TestConn
	outputDir         string
	contextAttributes map[string]string
	contextTags       map[ActionTag]struct{}
}

// UserAction represents the user action.
type UserAction struct {
	name               string
	action             action.Action
	userContext        *UserContext
	actionAttributes   map[string]string
	actionTags         map[ActionTag]struct{}
	validateResultFunc action.Action
	ifSuccessFunc      action.Action
	ifFailFunc         action.Action
	finalFunc          action.Action
}

// NewUserContext returns a new user context.
func NewUserContext(testName string, cr *chrome.Chrome, tconn *chrome.TestConn, outputDir string, contextAttributes map[string]string, contextTags []ActionTag) *UserContext {
	contextTagsMap := make(map[ActionTag]struct{})
	for _, tag := range contextTags {
		contextTagsMap[tag] = struct{}{}
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
func NewUserAction(name string, action action.Action, uc *UserContext, uac *UserActionCfg) *UserAction {
	return fromAction(name, action, uc, uac)
}

// NewUserAction returns a new user action.
func (uc *UserContext) NewUserAction(name string, action action.Action, uac *UserActionCfg) *UserAction {
	return fromAction(name, action, uc, uac)
}

// InvalidUserAction creates an user action to return error directly.
func (uc *UserContext) InvalidUserAction(err error) *UserAction {
	return fromAction("Invalid action", func(ctx context.Context) error {
		return err
	}, uc, nil)
}

// Refresh refreshes test name and output dir when sharing user context across tests.
// e.g. once UserContext is defined in precondition and fixture, the UserContext persistes across tests.
func (uc *UserContext) Refresh(testName, outputDir string) {
	uc.testName = testName
	uc.outputDir = outputDir
}

// SetAttribute set the value of an attribute of the user context.
func (uc *UserContext) SetAttribute(name, value string) {
	uc.contextAttributes[name] = value
}

// RemoveAttribute removes an attribute of the user context.
func (uc *UserContext) RemoveAttribute(name string) {
	delete(uc.contextAttributes, name)
}

// Attributes returns all attributes of the user context.
func (uc *UserContext) Attributes() map[string]string {
	return uc.contextAttributes
}

// AddTags adds tags to the user context.
func (uc *UserContext) AddTags(actionTags []ActionTag) {
	for _, newTag := range actionTags {
		uc.contextTags[newTag] = struct{}{}
	}
}

// RemoveTags removes tags from the user context.
func (uc *UserContext) RemoveTags(actionTags []ActionTag) {
	for _, tag := range actionTags {
		delete(uc.contextTags, tag)
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

// UserActionCfg represents optional configurations of an user action.
type UserActionCfg struct {
	ActionAttributes   map[string]string
	ActionTags         []ActionTag
	IfSuccessFunc      action.Action
	IfFailFunc         action.Action
	FinalFunc          action.Action
	ValidateResultFunc action.Action
}

func fromAction(name string, action action.Action, uc *UserContext, uac *UserActionCfg) *UserAction {
	if uac == nil {
		uac = &UserActionCfg{
			ActionAttributes: make(map[string]string),
			ActionTags:       []ActionTag{},
		}
	}

	actionTagsMap := make(map[ActionTag]struct{})
	for _, tag := range uac.ActionTags {
		actionTagsMap[tag] = struct{}{}
	}

	return &UserAction{
		name:               name,
		action:             action,
		userContext:        uc,
		actionAttributes:   uac.ActionAttributes,
		actionTags:         actionTagsMap,
		validateResultFunc: uac.ValidateResultFunc,
		ifSuccessFunc:      uac.IfSuccessFunc,
		ifFailFunc:         uac.IfFailFunc,
		finalFunc:          uac.FinalFunc,
	}
}

// RunAction runs a action.Action as an user action and records detailed running information.
func (uc *UserContext) RunAction(ctx context.Context, name string, action action.Action, uac *UserActionCfg) error {
	userAction := fromAction(name, action, uc, uac)
	return userAction.Run(ctx)
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
	combinedTagsMap := make(map[ActionTag]struct{})
	for k := range ua.userContext.contextTags {
		combinedTagsMap[k] = struct{}{}
	}
	for k := range ua.actionTags {
		combinedTagsMap[k] = struct{}{}
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
			duration:   endTime.Sub(startTime),
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
	if err == nil && ua.validateResultFunc != nil {
		err = ua.validateResultFunc(ctx)
	}

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
	ua.actionAttributes[key] = value
}

// AddTags adds tags to the user action.
func (ua *UserAction) AddTags(actionTags []ActionTag) {
	for _, newTag := range actionTags {
		ua.actionTags[newTag] = struct{}{}
	}
}

type actionResult struct {
	actionName string
	testName   string
	attributes map[string]string
	tags       []ActionTag
	duration   time.Duration
	pass       bool
	err        error
}

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
		fmt.Sprintf("%d", int64(ar.duration/time.Millisecond)),
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
