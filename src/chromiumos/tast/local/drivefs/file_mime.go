// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

// FileMime represents MIME type which is used in Google Drive.
// A reference for Google built-in files: https://developers.google.com/drive/api/guides/mime-types
// A reference for non-Google built-in files: https://developers.google.com/drive/api/guides/ref-export-formats
type FileMime string

// Google built-in file types.
const (
	// Folder is MIME type for folders.
	Folder FileMime = "application/vnd.google-apps.folder"

	// GoogleDoc is MIME type for Google documents.
	GoogleDoc FileMime = "application/vnd.google-apps.document"

	// GoogleSheets is MIME type for Google spreadsheets.
	GoogleSheets FileMime = "application/vnd.google-apps.spreadsheet"

	// GooglePresentation is MIME type for Google slides.
	GooglePresentation FileMime = "application/vnd.google-apps.presentation"

	// ThirdPartyShortcut is MIME type for third-party shortcuts.
	ThirdPartyShortcut FileMime = "application/vnd.google-apps.drive-sdk"

	// GoogleDrawing is MIME type for Google drawings.
	GoogleDrawing FileMime = "application/vnd.google-apps.drawing"

	// GoogleForm is MIME type for Google forms.
	GoogleForm FileMime = "application/vnd.google-apps.form"

	// GooglJamboard is MIME type for Google jamboard files.
	GooglJamboard FileMime = "application/vnd.google-apps.jam"

	// GoogleSite is MIME type for Google site files.
	GoogleSite FileMime = "application/vnd.google-apps.site"
)

// Non-Google built-in file types.
const (
	// MP3 is MIME type for MP3 files.
	MP3 FileMime = "audio/mpeg"

	// MP4 is MIME type for MP4 files.
	MP4 FileMime = "video/mp4"

	// HTML is MIME type for HTML file.
	HTML FileMime = "text/html"

	// Zip is MIME type for HTML file which is zipped.
	Zip FileMime = "application/zip"

	// PlainText is MIME type for plain text file.
	PlainText FileMime = "text/plain"

	// RichText is MIME type for rich text file.
	RichText FileMime = "application/rtf"

	// CSV is MIME type for "Comma-separated Values" file.
	CSV FileMime = "text/csv"

	// OpenOfficeDoc is MIME type for Open Office document.
	OpenOfficeDoc FileMime = "application/vnd.oasis.opendocument.text"

	// OpenOfficeSheet is MIME type for Open Office spreadsheet.
	OpenOfficeSheet FileMime = "application/x-vnd.oasis.opendocument.spreadsheet"

	// OpenOfficePresentation is MIME type for Open Office presentation.
	OpenOfficePresentation FileMime = "application/vnd.oasis.opendocument.presentation"

	// PDF is MIME type for PDF file.
	PDF FileMime = "application/pdf"

	// MsWordDoc is MIME type for Microsoft Word document.
	MsWordDoc FileMime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

	// MsExcelSheet is MIME type for Microsoft Excel spreadsheet.
	MsExcelSheet FileMime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

	// MsPowerpointPresentation is MIME type for Microsoft PowerPoint presentation.
	MsPowerpointPresentation FileMime = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

	// SheetOnly is a special format for Spreadsheets documents.
	SheetOnly FileMime = "text/tab-separated-values"

	// EPUB is MIME type for EPUB file.
	EPUB FileMime = "application/epub+zip"

	// JPEG is MIME type for JPEG file.
	JPEG FileMime = "image/jpeg"

	// PNG is MIME type for PNG file.
	PNG FileMime = "image/png"

	// SVG is MIME type for SVG file.
	SVG FileMime = "image/svg+xml"
)
