// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"chromiumos/tast/errors"
)

// DriveType is the type of the Google Drive file that we are
// are constructing a URL for.
type DriveType string

const (
	// DriveTypeDocs represents Google Docs.
	DriveTypeDocs DriveType = "docs"

	// DriveTypeSlides represents Google Slides
	DriveTypeSlides DriveType = "slides"

	// DriveTypeSheets represents Google Sheets.
	DriveTypeSheets DriveType = "sheets"
)

// driveFile contains information needed to randomly construct URLs for
// a single Google Drive file type. A url is constructed as follows:
// https://docs.google.com/<fileType>/d/<random id from ids>/edit
type driveFile struct {
	fileType string
	ids      []string
}

var (
	googleDocs = driveFile{
		fileType: "document",
		ids: []string{
			"1Q4xCQd2aVxwpIugEuGdVmWoNbdxkhc-ENTW9-frBMnQ",
			"169goDwL3s5nX-BxY0-qA2-DAtx9EKzQliaDa6dpKCV8",
			"1NB5Wbv0PuxT8zo-GT_uIyJZCY7FEvbWxBUnAX_vHta4",
			"1tE6oaSW875m2SMqMUEcnEcVcbt5cUCH75bfmeRGQ26o",
			"1pBorP7vprR8n4_QYizTURzl8BgaubYPuwfK2FoouH60",
			"1YprStoKZUweQ_zr0RuiAQKLOsf816p6j5mEPYokQUDU",
			"1uYRh_HMB8EJuU54UJlG-pwVUeCMRG3IsNsPhNRMHyic",
			"1_LeaFWWJQ4Y_6qETCESpRk_FeSbbLCWdyQ-LPaAkziw",
			"1DNU22wEApL5QNTLc7Gmy9uix8JwsdGGvRlE58Pj2Opc",
			"1IX0Nikx155xrY5vLIFbEME_BUrx-0cOWZWRLljbtmQc",
		},
	}
	googleSlides = driveFile{
		fileType: "presentation",
		ids: []string{
			"11-mh-vzb-ZEocqoafgU-KA1F1eql-p1K5kHYpbcLf3k",
			"1Jx_JHIWcrBSAF_PIrcFISQreGhprRm2gveQixOZEQio",
			"1yPZ0b6FpDTyUY92XjDQhC1KVmPw5U0mC1a0cY31-BuU",
			"1r7eOX9X3HOZRA_8s--Ay-kfKifZlmAGuZX6iI7qjReU",
			"1N4dKIoMKZBYVgdIRcp3fE-NH1g9nnMRbySKxs56ctX8",
			"1uev_ZfU6PgjPFMUCZOIJr0rxBdswzSG-8GYOJe6i90Y",
			"1P1fIp81hhVbWd5ZRw5v2U1eii3pebhTyXl2DQijbSBo",
			"16vRzSKAwx_QFV75H1t3GUrlq5EPit3Y5yNx_yWLZ3j8",
			"1-aSSvvDdnugUUfygj-opVUltSpZwJLae4IqiGTiMRc8",
			"1LmLydzA4tSYB6A1czpO1sUCWlMZ7QSp6CgtGOKRptBI",
		},
	}
	googleSheets = driveFile{
		fileType: "spreadsheets",
		ids: []string{
			"1Ij1sou7HIcydmLPR-s3xSe-cmap2H7lFhCErHX-HBmU",
			"1sKoKExkl_mpLF-HVz34Y0lpEIvaVFZzhB3FUpkJqY80",
			"1SVgMhg48r6LoZgXDbyNeG8_ZnVd1ZboYSMN2rWtQJ6c",
			"1IeiKPacIP4j6Wi1VbAyPGftEZlnr1hVNZRGwI8cjoyk",
			"16zIJDAn4zhKGH1sPKky7i6pXD7_yJSjzwbmG4jWYAJ4",
			"1TKUNSOINXg1mMM0_XkQ4oFXAd3CD--O_YyUv93U_Kio",
			"1hotZ9nrPEDzaBTzEfEayX5QwvGxarXoxNMG7x3CXzlM",
			"1r4Y1ASgTe-4-TBF93fXKMG3BR6hHBuKqi6FjFV9FeLw",
			"1OpqvOCdW-bQZTv_XXbb96UNAnFFN4P242Ae-pF1OE70",
			"1U9EJ9TXWnDH2CsBjR0t0jXpj2JK7HDcX9kE7J6V0vEY",
		},
	}
)

// GetDriveURL randomly chooses among ten copies of one of the
// following documents, dictated by |dt|, for tests to avoid heavy
// traffic which can cause a stripped down UI.
// - Google Docs:
// https://docs.google.com/document/d/1MW7lAk9RZ-6zxpObNwF0r80nu-N1sXo5f7ORG4usrJQ/edit
//
// - Google Slides
// https://docs.google.com/presentation/d/1lItrhkgBqXF_bsP-tOqbjcbBFa86--m3DT5cLxegR2k/edit
//
// - Google Sheets:
// https://docs.google.com/spreadsheets/d/1I9jmmdWkBaH6Bdltc2j5KVSyrJYNAhwBqMmvTdmVOgM/edit
func GetDriveURL(dt DriveType) (string, error) {
	var file driveFile
	switch dt {
	case DriveTypeDocs:
		file = googleDocs
	case DriveTypeSlides:
		file = googleSlides
	case DriveTypeSheets:
		file = googleSheets
	}

	i, err := rand.Int(rand.Reader, big.NewInt(int64(len(file.ids))))
	if err != nil {
		return "", errors.Wrap(err, "failed to get random number")
	}
	return fmt.Sprintf("https://docs.google.com/%s/d/%s/edit", file.fileType, file.ids[i.Uint64()]), nil
}

// GetTestDocCommentURL is like GetTestDocURL, but returns a link to a
// comment near the bottom of the document. This is useful for loading
// the entire document by scrolling to the bottom (or close enough).
func GetTestDocCommentURL() (string, error) {
	doc, err := GetDriveURL(DriveTypeDocs)
	if err != nil {
		return "", err
	}
	return doc + "?disco=AAAAP6EbSF8", nil
}
