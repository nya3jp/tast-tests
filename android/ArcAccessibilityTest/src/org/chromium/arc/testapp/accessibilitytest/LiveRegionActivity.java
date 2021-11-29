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

/**
 * Test Activity for arc.Accessibility* tast tests that involves live regions.
 */
public class LiveRegionActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.live_region_activity);

        setupLiveRegionButton(R.id.politeButton, R.id.politeText, "Updated Polite Text");
        setupLiveRegionButton(R.id.assertiveButton, R.id.assertiveText, "Updated Assertive Text");
    }

    private void setupLiveRegionButton(int buttonId, int textViewId, CharSequence text) {
        final Button button = findViewById(buttonId);
        button.setOnClickListener(
                view -> {
                    final TextView textView = findViewById(textViewId);
                    textView.setText(text);
                });
    }
}
