/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.preime;

import android.app.Activity;
import android.os.Bundle;
import android.text.InputType;
import android.widget.Button;
import android.widget.TextView;
import android.view.View;

public class MainActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        final CaptureKeyPreImeView field = (CaptureKeyPreImeView) findViewById(R.id.text);
        final CaptureKeyPreImeView nullEdit = (CaptureKeyPreImeView) findViewById(R.id.null_edit);
        nullEdit.setInputType(InputType.TYPE_NULL);
        final TextView lastPreImeKey = (TextView) findViewById(R.id.last_pre_ime_key);
        final TextView lastKeyDown = (TextView) findViewById(R.id.last_key_down);
        field.setLastKeyEventLabels(lastPreImeKey, lastKeyDown);
        nullEdit.setLastKeyEventLabels(lastPreImeKey, lastKeyDown);

        final Button startConsumingEvents = (Button) findViewById(R.id.start_consuming_events);
        startConsumingEvents.setOnClickListener((View v) -> {
            field.startConsumingEvents();
            nullEdit.startConsumingEvents();
            // Reset labels.
            lastPreImeKey.setText("null");
            lastKeyDown.setText("null");
          });
    }
}
