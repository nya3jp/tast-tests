// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"chromiumos/tast/errors"
)

// GetTestDocURL randomly chooses among ten copies of
// https://docs.google.com/document/d/1MW7lAk9RZ-6zxpObNwF0r80nu-N1sXo5f7ORG4usrJQ/edit
// for tests to avoid heavy traffic which can cause stripped down UI. Returns a link to
// a comment near the bottom of the document, so that the document will be scrolled to
// the bottom (or close enough) and therefore hopefully the entire document will load.
func GetTestDocURL() (string, error) {
	ids := [10]string{
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
	}
	i, err := rand.Int(rand.Reader, big.NewInt(int64(len(ids))))
	if err != nil {
		return "", errors.Wrap(err, "failed to get random number")
	}
	return fmt.Sprintf("https://docs.google.com/document/d/%s/edit?disco=AAAAP6EbSF8", ids[i.Uint64()]), nil
}
