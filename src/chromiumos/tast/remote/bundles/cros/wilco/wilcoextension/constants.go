// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wilcoextension contains constants and helpers to work with the
// extension that can interact with the Wilco DTC VM.
package wilcoextension

// ID is hardcoded in Chrome to have access to the private API.
const ID = "emelalhagcpibaiiiijjlkmhhbekaidg"

// Manifest gives the extension permissions necessary to access the private API.
const Manifest = `{
    "key": "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxcctELmluEjUimtjiCl44Z/U4iUm79M0lDBARqE0VmymkylGq4givCsZINZf2JyPJC1xM09UEpDvCyS/9qVISX8HzhuilCN1tNe8w473uiiIOHfgLwAYK5FO58F8G18LrkssKPM9OdCnvM+SVfgFxtcSZUxaiE0FRqs5hLmGSXnLSxDdFeVBUh9xQf2WdJL77inyBfP8pL9KnZ8CpKJ7HiK0ubAGBU4NkoDZRfWiGsKGYLIB/Lh3QWmymXrxVngoz+zqctoH/13Jt23civwyg7epnDwq48PurIU1zTs2bANtgsFc1sAEl2/bACAQHH7oX9ptgIL18Rt+8Khs+WghvwIDAQAB",
    "description": "Wilco test extension",
    "name": "Wilco test extension",
    "background": {
        "scripts": [
            "background.js"
        ]
    },
    "manifest_version": 2,
    "version": "0.1",
    "permissions": [
        "nativeMessaging"
    ]
}`
