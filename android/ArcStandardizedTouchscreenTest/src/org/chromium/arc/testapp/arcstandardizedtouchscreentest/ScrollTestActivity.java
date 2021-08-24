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

public class ScrollTestActivity extends Activity {

    private TextView mTxtScrollableContent;
    private TextView mTxtDebugState;

    // Determines how many pixels must be scrolled down before triggering the 'complete' text.
    static final int SCROLL_DOWN_THRESHOLD_AMOUNT = 100;

    // Determines how many pixels must be scrolled up before triggering the 'complete' text.
    // The up test happens after the down so it just needs to get below this amount.
    static final int SCROLL_UP_THRESHOLD_AMOUNT = 20;

    // The current state of the test
    private ScrollBeingTested mcurrentTestState = ScrollBeingTested.DOWN;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_scroll_test);

        mTxtDebugState = findViewById(R.id.txtDebugState);

        // Set up the scrollable content. The text size is adjusted to make sure
        // it always overflows.
        mTxtScrollableContent = findViewById(R.id.txtScrollableContent);
        mTxtScrollableContent.setText(R.string.scroll_content);
        mTxtScrollableContent.setTextSize(32.0f);
        mTxtScrollableContent.setMovementMethod(new ScrollingMovementMethod());
        mTxtScrollableContent.setOnScrollChangeListener(
                (v, scrollX, scrollY, oldScrollX, oldScrollY) -> {
                    // Always update the debug label.
                    mTxtDebugState.setText(String.format("scrollY: %d", scrollY));

                    // Determine the state of the test and update accordingly.
                    switch (mcurrentTestState) {
                        case DOWN:
                            if (scrollY > SCROLL_DOWN_THRESHOLD_AMOUNT) {
                                // Update the UI to show that the down test is finished and start
                                // the up test.
                                TextView txtScrollDownTestState =
                                        findViewById(R.id.txtScrollDownTestState);
                                txtScrollDownTestState.setText("COMPLETE");
                                mcurrentTestState = ScrollBeingTested.UP;
                            }
                            break;
                        case UP:
                            if (scrollY < SCROLL_UP_THRESHOLD_AMOUNT) {
                                // Update the UI and mark the tests as done.
                                TextView txtScrollUpTestState =
                                        findViewById(R.id.txtScrollUpTestState);
                                txtScrollUpTestState.setText("COMPLETE");
                                mcurrentTestState = ScrollBeingTested.NONE;
                            }
                            break;
                    }
                });
    }
}

enum ScrollBeingTested {
    NONE,
    DOWN,
    UP
}
