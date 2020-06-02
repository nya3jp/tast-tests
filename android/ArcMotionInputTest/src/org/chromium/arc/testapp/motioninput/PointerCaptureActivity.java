/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.motioninput;

import android.os.Bundle;
import android.util.Log;
import android.view.MotionEvent;
import android.view.View;
import android.widget.TextView;
import org.json.JSONException;
import org.json.JSONObject;

/**
 * PointerCaptureActivity is a {@link MotionEventReportingActivity} that requests Pointer Capture to
 * be enabled whenever the activity is clicked, and reports all {@link MotionEvent}s that it
 * receives, including captured events.
 */
public class PointerCaptureActivity extends MotionEventReportingActivity {

    private View mCaptureView;
    private TextView mTvPointerCaptureState;

    private static final String KEY_POINTER_CAPTURE_STATE = "pointer_capture_enabled";

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        mTvPointerCaptureState = findViewById(R.id.pointer_capture_state);
        updatePointerCaptureState(false);

        mCaptureView = findViewById(R.id.capture_view);
        mCaptureView.setOnCapturedPointerListener((view, event) -> {
            reportMotionEvent(event);
            return true;
        });
        mCaptureView.setOnClickListener(View::requestPointerCapture);
    }

    @Override
    public void onPointerCaptureChanged(boolean hasCapture) {
        updatePointerCaptureState(hasCapture);
    }

    protected void updatePointerCaptureState(boolean enabled) {
        final JSONObject stateObject = new JSONObject();
        try {
            stateObject.put(KEY_POINTER_CAPTURE_STATE, enabled);
        } catch (JSONException e) {
            Log.e(TAG, "Failed to write event to JSON", e);
        }
        mTvPointerCaptureState.setText(stateObject.toString());
    }
}
