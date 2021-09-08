/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedinputtest;

import android.app.Activity;
import android.os.Bundle;
import android.view.ScaleGestureDetector;
import android.widget.TextView;

public class ZoomTestActivity extends Activity {
    private ScaleGestureDetector mZoomDetector;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_zoom_test);

        // Setup the zoom detector.
        TextView txtZoom = findViewById(R.id.txtZoom);
        mZoomDetector =
                new ScaleGestureDetector(
                        txtZoom.getContext(),
                        new ZoomGestureListener(
                                scaleFactor -> {
                                    TextView txtZoomInState = findViewById(R.id.txtZoomInState);
                                    txtZoomInState.setText("ZOOM IN: COMPLETE");
                                },
                                scaleFactor -> {
                                    TextView txtZoomOutState = findViewById(R.id.txtZoomOutState);
                                    txtZoomOutState.setText("ZOOM OUT: COMPLETE");
                                },
                                scaleFactor -> {
                                    TextView txtDebugPreviousZoom =
                                            findViewById(R.id.txtDebugPreviousZoom);
                                    txtDebugPreviousZoom.setText(
                                            String.format(
                                                    "DEBUG PREVIOUS ZOOM: SCALE FACTOR: %s",
                                                    scaleFactor));
                                }));

        // Send touch events to the zoom detector.
        txtZoom.setOnTouchListener(
                (v, event) -> {
                    mZoomDetector.onTouchEvent(event);
                    return true;
                });
    }
}
