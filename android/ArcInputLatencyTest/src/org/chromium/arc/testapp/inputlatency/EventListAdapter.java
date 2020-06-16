/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputlatency;

import android.content.Context;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
import android.widget.BaseAdapter;
import android.widget.TextView;

import java.util.ArrayList;

class EventListAdapter extends BaseAdapter {
    private Context context;
    private ArrayList<ReceivedEvent> list = new ArrayList<>();

    public EventListAdapter(Context context) {
        this.context = context;
    }

    public void add(ReceivedEvent item) {
        list.add(0, item);
        notifyDataSetChanged();
    }

    @Override
    public View getView(int position, View convertView, ViewGroup parent) {
        View view = convertView;
        if (view == null)
            view = LayoutInflater.from(context).inflate(R.layout.event_list_item, null);
        ReceivedEvent item = getItem(position);
        ((TextView) view.findViewById(R.id.event_type)).setText(item.type);
        ((TextView) view.findViewById(R.id.kernel_time)).setText(item.kernelTime.toString());
        ((TextView) view.findViewById(R.id.app_time)).setText(item.receiveTime.toString());
        ((TextView) view.findViewById(R.id.latency)).setText(item.latency.toString());
        return view;
    }

    @Override
    public ReceivedEvent getItem(int position) {
        return list.get(position);
    }

    @Override
    public long getItemId(int position) {
        return list.get(position).event.getEventTime();
    }

    @Override
    public int getCount() {
        return list.size();
    }
}
