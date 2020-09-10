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
import android.view.InputDevice;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.View;
import android.widget.Button;
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
    private EventListAdapter mAdapter;
    private ListView mList;
    private TextView mEvents;
    private TextView mCount;
    private ExecutorService mExecutor = Executors.newSingleThreadExecutor();
    private ArrayList<ReceivedEvent> mRecvEvents = new ArrayList<>();

    // Finish trace and save the events as JSON to TextView UI.
    private void finishTrace() {
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

    private void clearUI() {
        mRecvEvents.clear();
        mAdapter.notifyDataSetChanged();
        mExecutor.submit(
            () -> {
              runOnUiThread(() -> setEvents("", 0));
            });
    }

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
        // ESC key is a sign to finish tracing.
        if(event.getKeyCode() == KeyEvent.KEYCODE_ESCAPE) {
            finishTrace();
            return false;
        }
        if(event.getKeyCode() == KeyEvent.KEYCODE_DEL) {
            clearUI();
            return false;
        }
        ReceivedEvent recv =
                new ReceivedEvent(event, SystemClock.uptimeMillis(), System.currentTimeMillis());

        traceEvent(recv);
        final int source = event.getSource();
        // Stop dispatching gamepad event.
        if ((event.getSource() & InputDevice.SOURCE_GAMEPAD) != 0
                || (source & InputDevice.SOURCE_JOYSTICK) != 0) {
            return true;
        }
        return super.dispatchKeyEvent(event);
    }

    @Override
    public boolean dispatchTouchEvent(MotionEvent event) {
        traceEvent(
                new ReceivedEvent(event, SystemClock.uptimeMillis(), System.currentTimeMillis()));
        final int source = event.getSource();
        // Stop dispatching gamepad event.
        if ((source & InputDevice.SOURCE_GAMEPAD) != 0
                || (source & InputDevice.SOURCE_JOYSTICK) != 0) {
            return true;
        }
        return super.dispatchTouchEvent(event);
    }

    @Override
    public boolean dispatchGenericMotionEvent(MotionEvent event) {
        final int source = event.getSource();
        // Dispatching non-gamepad event.
        if ((source & InputDevice.SOURCE_GAMEPAD) == 0
                && (source & InputDevice.SOURCE_JOYSTICK) == 0) {
            traceEvent(
                new ReceivedEvent(event, SystemClock.uptimeMillis(), System.currentTimeMillis()));
            return  super.dispatchGenericMotionEvent(event);
        }
        // It is possible that the InputDevice is already gone when MotionEvent arrives.
        InputDevice device = InputDevice.getDevice(event.getDeviceId());
        if (device == null) {
            return true;
        }
        traceEvent(
                new ReceivedEvent(event, SystemClock.uptimeMillis(), System.currentTimeMillis()));
        return true;
    }

    /** Called to record timing of received input events. */
    private void traceEvent(ReceivedEvent recv) {
        // Ignore TouchScreen event that is ACTION_CANCEL
        if (recv.source == "Touchscreen" && recv.action == "ACTION_CANCEL") {
            return;
        }

        Log.v(TAG, recv.toString());
        mRecvEvents.add(recv);
        mAdapter.notifyDataSetChanged();

        // Record the event numbers.
        mExecutor.submit(
            () -> {
              runOnUiThread(() -> setEvents("", mRecvEvents.size()));
            });
    }

    private void setEvents(String json, Integer count) {
        mEvents.setText(json);
        mCount.setText(count.toString());
    }
}
