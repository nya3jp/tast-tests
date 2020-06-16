/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputlatency;

import android.app.Activity;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;
import android.view.InputEvent;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.widget.ListView;

/** Main activity for the ArcInputLatencyTest app. */
public class MainActivity extends Activity {
    private static final String TAG = "InputLatencyTest";
    private EventListAdapter adapter;
    private ListView list;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        adapter = new EventListAdapter(getApplicationContext());
        list = findViewById(R.id.event_list);
        list.setAdapter(adapter);
    }

    @Override
    public boolean dispatchKeyEvent(KeyEvent event) {
        traceEvent(event, SystemClock.uptimeMillis());
        return super.dispatchKeyEvent(event);
    }

    @Override
    public boolean dispatchTouchEvent(MotionEvent event) {
        traceEvent(event, SystemClock.uptimeMillis());
        return super.dispatchTouchEvent(event);
    }

    public void traceEvent(InputEvent event, long now) {
        if (event != null) {
            ReceivedEvent recv = new ReceivedEvent(event, now);
            adapter.add(recv);
            list.setSelectionAfterHeaderView();
            // TODO(wvk): Once there is ARC tracing support in R, we can use
            // atrace instead of logcat;
            Log.i(TAG, recv.toString());
        }
    }
}
