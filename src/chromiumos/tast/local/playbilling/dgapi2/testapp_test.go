// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dgapi2

import (
	"testing"
)

func TestVerifyGetDetailsLogs_OK(t *testing.T) {
	for _, logs := range [][]string{
		{
			"getDetails returned:",
			`{"description":"These are 100 of the finest digital coins that money can buy.","iconURLs":[""],"itemId":"coins_100","price":{"currency":"USD","value":"0.990000"},"title":"100 Coins (Stock Potatoh App)","type":"product","purchaseType":"repeatable"}`,
			`{"status":"Purchase was already acknowledged"}`,
			"OperationError: clientAppUnavailable",
		},
		{
			"getDetails returned:",
			`{"description":"These are 100 of the finest digital coins that money can buy.","iconURLs":[""],"itemId":"coins_100","price":{"currency":"USD","value":"0.990000"},"title":"100 Coins (Stock Potatoh App)","type":"product","purchaseType":"repeatable"}`,
			"listPurchaseHistory returned:",
		},
		{
			"getDetails returned:",
			`{"status":"Purchase was already acknowledged"}`,
			"listPurchaseHistory returned:",
		},
	} {
		if err := verifyGetDetailsLogs(logs); err != nil {
			t.Errorf("verifyGetDetailsLogs(%v) = %v; want nil", logs, err)
		}
	}
}

func TestVerifyGetDetailsLogs_FAIL(t *testing.T) {
	for _, logs := range [][]string{
		// No logs.
		{},
		{
			// Correct sku json, missing expected start and finish lines.
			`{"description":"These are 100 of the finest digital coins that money can buy.","iconURLs":[""],"itemId":"coins_100","price":{"currency":"USD","value":"0.990000"},"title":"100 Coins (Stock Potatoh App)","type":"product","purchaseType":"repeatable"}`,
		},
		{
			// Empty sku json.
			"getDetails returned:",
			"{}",
			"listPurchaseHistory returned:",
		},
		{
			// Missing sku json.
			"getDetails returned:",
			"listPurchaseHistory returned:",
		},
		{
			// Incorrect sku json.
			"getDetails returned:",
			"{this-is-incorrect-json}",
			"listPurchaseHistory returned:",
		},
		{
			// Sku json missing itemId field.
			"getDetails returned:",
			`{"description":"These are 100 of the finest digital coins that money can buy.","iconURLs":[""],"price":{"currency":"USD","value":"0.990000"},"title":"100 Coins (Stock Potatoh App)","type":"product","purchaseType":"repeatable"}`,
			"listPurchaseHistory returned:",
		},
		{
			// Sku json missing title field.
			"getDetails returned:",
			`{"description":"These are 100 of the finest digital coins that money can buy.","iconURLs":[""],"itemId":"coins_100","price":{"currency":"USD","value":"0.990000"},"type":"product","purchaseType":"repeatable"}`,
			"listPurchaseHistory returned:",
		},
		{
			// Sku json missing currency field.
			"getDetails returned:",
			`{"description":"These are 100 of the finest digital coins that money can buy.","iconURLs":[""],"itemId":"coins_100","price":{"value":"0.990000"},"type":"product","purchaseType":"repeatable"}`,
			"listPurchaseHistory returned:",
		},
		{
			// Sku json missing value field.
			"getDetails returned:",
			`{"description":"These are 100 of the finest digital coins that money can buy.","iconURLs":[""],"itemId":"coins_100","price":{"currency":"USD"},"title":"100 Coins (Stock Potatoh App)","type":"product","purchaseType":"repeatable"}`,
			"listPurchaseHistory returned:",
		},
	} {
		if err := verifyGetDetailsLogs(logs); err == nil {
			t.Errorf("verifyGetDetailsLogs(%v) = nil; want error", logs)
		}
	}
}
