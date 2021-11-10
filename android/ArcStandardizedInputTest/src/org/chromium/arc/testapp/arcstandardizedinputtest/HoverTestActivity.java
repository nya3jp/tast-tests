/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedinputtest;

import android.app.Activity;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.os.Bundle;
import android.view.MotionEvent;
import android.widget.TextView;

public class HoverTestActivity extends Activity {
    // Intent which starts the hover test.
    private static final String ACTION_START_HOVER_TEST =
            "org.chromium.arc.testapp.arcstandardizedinputtest.ACTION_START_HOVER_TEST";

    private boolean mIsTestStarted = false;

    // Watches for broadcasted intents to respond to.
    BroadcastReceiver mReceiver =
            new BroadcastReceiver() {
                @Override
                public void onReceive(Context context, Intent intent) {
                    switch (intent.getAction()) {
                        case ACTION_START_HOVER_TEST:
                            mIsTestStarted = true;
                            TextView txtStatus = findViewById(R.id.txtStatus);
                            txtStatus.setText("Status: Started");
                            break;
                        default:
                            // Do nothing
                            break;
                    }

                    setResultCode(Activity.RESULT_OK);
                }
            };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_hover_test);

        // Setup a broadcast receiver to allow the test state to be managed externally.
        IntentFilter intentFilter = new IntentFilter();
        intentFilter.addAction(ACTION_START_HOVER_TEST);
        registerReceiver(mReceiver, intentFilter);

        TextView txtToHover = findViewById(R.id.txtToHover);
        txtToHover.setOnHoverListener(
                (hoverView, hoverEvent) -> {
                    // Make sure the test was started before handling any events.
                    if (!mIsTestStarted) {
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
