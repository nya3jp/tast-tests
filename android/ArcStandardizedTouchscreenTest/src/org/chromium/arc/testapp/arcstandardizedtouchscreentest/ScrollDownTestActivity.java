/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedtouchscreentest;

import android.app.Activity;
import android.os.Bundle;
import android.text.method.ScrollingMovementMethod;
import android.widget.TextView;

public class ScrollDownTestActivity extends Activity {

    private TextView mTxtScrollableContent;

    // Determines how many pixels must be scrolled before triggering the 'complete' text.
    static final int COMPLETE_TEST_SCROLL_THRESHOLD = 100;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_scroll_down_test);

        // Set up the scrollable content. The text size is adjusted to make sure
        // it always overflows.
        mTxtScrollableContent = findViewById(R.id.txtScrollableContent);
        mTxtScrollableContent.setText(R.string.scroll_content);
        mTxtScrollableContent.setTextSize(32.0f);
        mTxtScrollableContent.setMovementMethod(new ScrollingMovementMethod());
        mTxtScrollableContent.setOnScrollChangeListener(
                (v, scrollX, scrollY, oldScrollX, oldScrollY) -> {
                    TextView txtTestState = findViewById(R.id.txtTestState);
                    if (scrollY > COMPLETE_TEST_SCROLL_THRESHOLD) {
                        // Mark the test as 'COMPLETE' after scrolling past the threshold.
                        txtTestState.setText("COMPLETE");
                    } else {
                        // Otherwise set to pending with a debug label.
                        txtTestState.setText(String.format("PENDING - scrollY: %d", scrollY));
                    }
                });
    }
}
