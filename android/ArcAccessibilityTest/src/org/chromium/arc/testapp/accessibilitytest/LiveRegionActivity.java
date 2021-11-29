/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.accessibilitytest;

import android.app.Activity;
import android.os.Bundle;
import android.widget.Button;
import android.widget.TextView;

/** Test Activity for arc.Accessibility* tast tests that involves live regions. */
public class LiveRegionActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.live_region_activity);

        setupLiveRegionButton(
                findViewById(R.id.politeButton),
                findViewById(R.id.politeText),
                "Updated polite text");
        setupLiveRegionButton(
                findViewById(R.id.assertiveButton),
                findViewById(R.id.assertiveText),
                "Updated assertive text");
    }

    private static void setupLiveRegionButton(Button button, TextView textView, CharSequence text) {
        button.setOnClickListener(v -> textView.setText(text));
    }
}
