/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.sensorlatency;

import android.app.Activity;
import android.content.Context;
import android.hardware.Sensor;
import android.hardware.SensorEvent;
import android.hardware.SensorEventListener;
import android.hardware.SensorManager;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;
import android.widget.Button;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;

class ReceivedEvent {
    long timestamp;
    long recvTime;

    public ReceivedEvent(long timestamp, long recvTime) {
        this.timestamp = timestamp;
        this.recvTime = recvTime;
    }
}

/** Main activity for the ArcSensorLatencyTest app. */
public class MainActivity extends Activity implements SensorEventListener {
    private static final String TAG = "SensorLatencyTest";
    private boolean mIsRecording = false;
    private long mCount = 0;
    private HashMap<Sensor, List<ReceivedEvent>> mEvents = new HashMap<>();
    private SensorManager mSensorManager;
    private TextView mCountView;
    private TextView mResultsView;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        final Button startButton = findViewById(R.id.start_button);
        startButton.setOnClickListener(v -> startListening());
        final Button stopButton = findViewById(R.id.stop_button);
        stopButton.setOnClickListener(v -> stopListening());

        mCountView = findViewById(R.id.count);
        mResultsView = findViewById(R.id.results);

        mSensorManager = (SensorManager) getSystemService(Context.SENSOR_SERVICE);
    }

    @Override
    public void onPause() {
        super.onPause();
        // Don't listen to sensors while in the background to reduce power consumption.
        if (mIsRecording) stopListening();
    }

    public void clear() {
        mEvents.clear();
        mCount = 0;
        mCountView.setText("0");
        mResultsView.setText("");
    }

    public void startListening() {
        clear();
        // Listen to all continuous sensors, at the fastest sampling rate possible.
        for (Sensor sensor : mSensorManager.getSensorList(Sensor.TYPE_ALL)) {
            if (sensor.getReportingMode() != Sensor.REPORTING_MODE_CONTINUOUS) continue;

            Log.i(
                    TAG,
                    "Registering as listener for sensor "
                            + sensor.getName()
                            + ":"
                            + sensor.getStringType()
                            + ":"
                            + sensor.getId());
            mSensorManager.registerListener(this, sensor, SensorManager.SENSOR_DELAY_FASTEST);
            mEvents.put(sensor, new ArrayList<ReceivedEvent>());
        }
        mIsRecording = true;
    }

    public void stopListening() {
        Log.i(TAG, "Unregistering all listeners");
        mSensorManager.unregisterListener(this);
        mIsRecording = false;
        reportData();
    }

    @Override
    public void onAccuracyChanged(Sensor sensor, int accuracy) {
        Log.i(TAG, "Sensor " + sensor.getName() + " accuracy changed to " + accuracy);
    }

    @Override
    public void onSensorChanged(SensorEvent event) {
        long recvTime = SystemClock.elapsedRealtimeNanos();

        Sensor sensor = event.sensor;
        if (!mEvents.containsKey(sensor)) {
            Log.e(TAG, "Received unexpected SensorEvent: " + event.toString());
            return;
        }

        ReceivedEvent recv = new ReceivedEvent(event.timestamp, recvTime);
        mEvents.get(sensor).add(recv);
        incrementCount();
    }

    public void incrementCount() {
        ++mCount;
        // Only update view every 1000 events. Updating the view more than this
        // can cause UIAutomator to hang.
        if (mCount % 1000 != 0) return;
        mCountView.setText(Long.toString(mCount));
    }

    public void reportData() {
        try {
            JSONArray results = new JSONArray();
            for (Sensor sensor : mEvents.keySet()) {
                JSONObject obj = new JSONObject();
                obj.put("name", sensor.getName());
                obj.put("type", sensor.getStringType());

                // Calculate average latency of events from this sensor device and add to JSON
                // object.
                List<ReceivedEvent> events = mEvents.get(sensor);
                obj.put("numEvents", events.size());
                double avgLatencyNs =
                        events.stream()
                                .mapToLong(e -> e.recvTime - e.timestamp)
                                .average()
                                .getAsDouble();
                obj.put("avgLatencyNs", avgLatencyNs);

                // Calculate average delay between events.
                double sum = 0;
                for (int i = 0; i < events.size() - 1; i += 2) {
                    sum += events.get(i + 1).timestamp - events.get(i).timestamp;
                }
                double avgDelayNs = sum / events.size();
                obj.put("avgDelayNs", avgDelayNs);

                results.put(obj);
            }
            mResultsView.setText(results.toString(/* indentSpaces */ 2));
        } catch (JSONException e) {
            Log.e(TAG, "Unable to report latency results: ", e);
        }
    }
}
