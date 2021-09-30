// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package productivitycuj

import (
	"context"
	"time"
)

const (
	// MicrosoftWeb indicates testing against Microsoft Web.
	MicrosoftWeb = "MicrosoftWeb"
	// GoogleWeb indicates testing against Google Web.
	GoogleWeb = "GoogleWeb"
)

const (
	// docText indicates content written as a paragraph of the "Microsoft Word" or "Google Docs".
	docText = "Copy to spreadsheet"
	// titleText indicates content written as a title of "Microsoft PowerPoint" or "Google Slides".
	titleText = "CUJ title"
	// subtitleText indicates content written as a subtitle of "Microsoft PowerPoint" or "Google Slides".
	subtitleText = "CUJ subtitle"

	// sheetName indicates the name of the existing spreadsheet.
	sheetName = "sample"

	// rangeOfCells indicates the sum of rows in the spreadsheet.
	rangeOfCells = 100

	// defaultUIWaitTime indicates the default time to wait for UI elements to appear.
	defaultUIWaitTime = 5 * time.Second
	// defaultUIWaitTime indicates the time to wait for some UI elements that need more time to appear.
	longerUIWaitTime = time.Minute

	// retryTimes defines the key UI operation retry times.
	retryTimes = 3
)

// ProductivityApp contains user's operation in productivity application.
type ProductivityApp interface {
	CreateDocument(ctx context.Context) error
	CreateSlides(ctx context.Context) error
	CreateSpreadsheet(ctx context.Context) (string, error)
	OpenSpreadsheet(ctx context.Context, filename string) error
	MoveDataFromDocToSheet(ctx context.Context) error
	MoveDataFromSheetToDoc(ctx context.Context) error
	ScrollPage(ctx context.Context) error
	SwitchToOfflineMode(ctx context.Context) error
	UpdateCells(ctx context.Context) error
	VoiceToTextTesting(ctx context.Context, expectedText string, playAudio func(context.Context) error) error
	End(ctx context.Context) error
}
