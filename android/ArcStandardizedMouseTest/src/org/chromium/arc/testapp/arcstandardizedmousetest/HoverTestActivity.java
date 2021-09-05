/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedmousetest;

import android.app.Activity;
import android.os.Bundle;
import android.view.MotionEvent;
import android.widget.Button;
import android.widget.TextView;

public class HoverTestActivity extends Activity {
    private boolean mIsTestStarted = false;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_hover_test);

        // Don't start the test until the button is clicked. This helps to ensure that the mouse
        // isn't in a random location at the start of the test, which could cause the events to
        // trigger
        // before the test was ready.
        Button btnStartHoverTest = findViewById(R.id.btnStartHoverTest);
        btnStartHoverTest.setOnClickListener(
                v -> {
                    mIsTestStarted = true;
                });

        TextView txtToHover = findViewById(R.id.txtToHover);
        txtToHover.setOnHoverListener(
                (hoverView, hoverEvent) -> {
                    // Make sure the test was started before handling any events.
                    if (mIsTestStarted == false) {
                        return true;
                    }

                    // Update the corresponding state based on the event.
                    switch (hoverEvent.getAction()) {
                        case MotionEvent.ACTION_HOVER_ENTER:
                            TextView txtHoverEnterState = findViewById(R.id.txtHoverEnterState);
                            txtHoverEnterState.setText("HOVER ENTER: COMPLETE");
                            break;
                        case MotionEvent.ACTION_HOVER_EXIT:
                            TextView txtHoverExitState = findViewById(R.id.txtHoverExitState);
                            txtHoverExitState.setText("HOVER EXIT: COMPLETE");
                            break;
                        default:
                            // Do nothing.
                            break;
                    }

                    return true;
                });
    }
}
