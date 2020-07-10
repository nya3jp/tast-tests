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
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.widget.ListView;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONException;

import java.util.ArrayList;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

/** Main activity for the ArcInputLatencyTest app. */
public class MainActivity extends Activity {
    private static final String TAG = "InputLatencyTest";
    private static final int FINISH_KEY = KeyEvent.KEYCODE_ESCAPE;
    private EventListAdapter mAdapter;
    private ListView mList;
    private TextView mEvents;
    private TextView mCount;
    private ExecutorService mExecutor = Executors.newSingleThreadExecutor();
    private ArrayList<ReceivedEvent> mRecvEvents = new ArrayList<>();
    private Float mLastMouseX = null;
    private Float mLastMouseY = null;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        mAdapter = new EventListAdapter(getApplicationContext(), mRecvEvents);
        ((ListView) findViewById(R.id.event_list)).setAdapter(mAdapter);
        mEvents = findViewById(R.id.event_json);
        mCount = findViewById(R.id.event_count);
    }

    @Override
    public void onStop() {
        // Wait up to 5 seconds for the remaining jobs in the queue to finish.
        try {
            mExecutor.awaitTermination(5, TimeUnit.SECONDS);
        } catch (InterruptedException e) {
            Log.e(
                    TAG,
                    "thread was interrupted while waiting for executor termination: "
                            + e.getMessage());
        }
        super.onStop();
    }

    @Override
    public boolean dispatchKeyEvent(KeyEvent event) {
        ReceivedEvent recv =
                new ReceivedEvent(event, SystemClock.uptimeMillis(), System.currentTimeMillis());

        traceEvent(recv);
        return super.dispatchKeyEvent(event);
    }

    @Override
    public boolean dispatchTouchEvent(MotionEvent event) {
        traceEvent(
                new ReceivedEvent(event, SystemClock.uptimeMillis(), System.currentTimeMillis()));
        return super.dispatchTouchEvent(event);
    }

    @Override
    public boolean dispatchGenericMotionEvent(MotionEvent event) {
        traceEvent(
                new ReceivedEvent(event, SystemClock.uptimeMillis(), System.currentTimeMillis()));
        return super.dispatchGenericMotionEvent(event);
    }

    /** Called to record timing of received input events. */
    private void traceEvent(ReceivedEvent recv) {
        // Android may sometimes send duplicate MotionEvent events with the same coordinates
        // and action as the last event. Discard these events.
        if (recv.type == "MouseEvent") {
            MotionEvent event = (MotionEvent) recv.event;
            if ((mLastMouseX != null && mLastMouseY != null)
                    && mLastMouseX.equals(event.getX(0))
                    && mLastMouseY.equals(event.getY(0))) {
                // ignore this event
                return;
            }
            mLastMouseX = event.getX(0);
            mLastMouseY = event.getY(0);
        }

        mRecvEvents.add(recv);
        mAdapter.notifyDataSetChanged();

        // Serialize events to JSON
        mExecutor.submit(
                () -> {
                    JSONArray arr = new JSONArray();
                    try {
                        for (ReceivedEvent ev : mRecvEvents) {
                            arr.put(ev.toJSON());
                        }
                        String json = arr.toString();
                        int len = arr.length();
                        runOnUiThread(() -> setEvents(json, len));

                    } catch (JSONException e) {
                        Log.e(TAG, "Unable to serialize events to JSON: " + e.getMessage());
                    }
                });
    }

    private void setEvents(String json, Integer count) {
        mEvents.setText(json);
        mCount.setText(count.toString());
    }
}
