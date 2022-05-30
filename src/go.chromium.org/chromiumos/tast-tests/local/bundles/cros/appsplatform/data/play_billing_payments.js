// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

const PAYMENT_METHOD = "https://play.google.com/billing";

/**
 * Attempts to make a purchase of the supplied sku.
 *
 * @param {string} sku The unit code to purchase.
 */
function buy(sku) {
    if (!window.PaymentRequest) {
        console.log("No PaymentRequest object.");
    }

    const supportedInstruments = [{
        supportedMethods: PAYMENT_METHOD,
        data: {
            sku: sku,
        },
    }];

    const item_details = {
        total: {
            label: "Total",
            amount: {
                currency: "AUD",
                value: "1",
            },
        },
    };

    const request = new PaymentRequest(supportedInstruments, item_details);

    if (request.canMakePayment) {
        request.canMakePayment()
            .then(result => {
                console.log(result ?
                    "Can make payment." :
                    "Cannot make payment.");
            })
            .catch(error => console.error(error.message));
    }

    if (request.hasEnrolledInstrument) {
        request.hasEnrolledInstrument()
            .then(result => {
                console.log(result ?
                    "Has enrolled instrument." :
                    "No enrolled instrument.");

                request.show().then((response) => {
                    console.log('payment successful');
                    response.complete('success');
                });
            })
            .catch(error => console.error(error.message));
    }
}
