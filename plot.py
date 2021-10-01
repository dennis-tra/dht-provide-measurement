import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import plotly.graph_objects as go

events = pd.read_csv('plot_data.csv')

# plt.xlim(0, 90000)
# for index, row in events.iterrows():
#     distance = int(row['distance'], base=16)
#     distance_norm = distance / 2**256
#     plt.axhline(y=distance_norm, xmin=row['start']/90000, xmax=row['end']/90000, color=row['color'], linestyle='-')
# plt.yscale('log')
# plt.show()
# import plotly.express as px
# data_canada = px.data.gapminder().query("country == 'Canada'")
# fig = px.bar(data_canada, x='year', y='pop')
# fig.add_hline(y=0.9)
# fig.show()

import plotly.express as px

norm_distance = []
for index, row in events.iterrows():
    distance = int(row['distance'], base=16)
    norm_distance += [distance / 2 ** 256]
events['norm_distance'] = norm_distance

# events = events[events["type"] != "*main.OpenStreamStart"]
# events = events[events["type"] != "*main.OpenStreamEnd"]
events = events[events["type"] != "*main.OpenedStream"]
events = events[events["type"] != "*main.ClosedStream"]
events = events[events["type"] != "*main.ConnectedEvent"]
events = events[events["type"] != "*main.DisconnectedEvent"]

fig = go.Figure()

min_times = {}
for index, row in events.iterrows():
    peer_id = row["peer_id"]
    time = row["time"]
    if peer_id in min_times:
        if time < min_times[peer_id]:
            min_times[peer_id] = time
    else:
        min_times[peer_id] = time

peers = []
times = []
for key in min_times:
    peers += [key]
    times += [min_times[key]]

ordered_peers = [x for _, x in sorted(zip(times, peers))]
peer_order = {}
i = 0
for peer in ordered_peers:
    peer_order[peer] = len(ordered_peers) - i
    i += 1


def track_start_event(states, peer_id):
    if peer_id not in states:
        states[peer_id] = {
            "time": time,
            "counter": 1,
        }
    else:
        states[peer_id]["counter"] += 1


def handle_end_event(states, event, color_success, color_error, name, offset):
    peer_id = event["peer_id"]
    event_type = event["type"]
    time = event["time"]
    has_error = event["has_error"]
    error = event["error"]
    extra = event["extra"]

    if peer_id not in states:
        return

    states[peer_id]["counter"] -= 1

    if event["has_error"] and states[peer_id]["counter"] != 0:
        return

    if event["time"] - states[peer_id]["time"] < 0.001:  # don't render dial shorter than 1ms
        states.pop(row["peer_id"])
        return

    color = color_error if has_error else color_success

    trace = {
        'time': [states[peer_id]["time"], time],
        'norm_distance': [row["norm_distance"], row["norm_distance"]],
        'order': [peer_order[peer_id]+offset, peer_order[peer_id]+offset],
    }

    duration_in_s = time - states[peer_id]["time"]
    duration_str = "{:.3f}s".format(duration_in_s) if duration_in_s < 1 else "{:.1f}ms".format(duration_in_s * 1000)

    fig.add_trace(go.Scatter(
        x=trace["time"],
        y=trace["order"],
        name=name,
        marker=dict(
            size=5,
            color=[color, color]
        ),
        line=dict(
            color=color,
            width=5
        ),
        hovertemplate="Duration: {:s}<br>Peer ID: {:s}<br>Error: {:s}<br>Extra: {:s}".format(
            duration_str, peer_id, str(error)[:30] if has_error else "-", str(extra)),
        showlegend=False
    ))
    states.pop(peer_id)


dial_states = {}
message_states = {}
request_states = {}
stream_states = {}

for index, row in events.iterrows():
    peer_id = row["peer_id"]
    event_type = row["type"]
    time = row["time"]
    has_error = row["has_error"]
    error = row["error"]
    extra = row["extra"]

    if event_type == "*main.SendRequestStart":
        track_start_event(request_states, peer_id)

    elif event_type == "*main.SendRequestEnd":
        handle_end_event(request_states, row, "green", "lightgreen", "Finding Closer Nodes", 0.45)

    elif event_type == "*main.DialStart":
        track_start_event(dial_states, peer_id)

    elif event_type == "*main.DialEnd":
        handle_end_event(dial_states, row, "red", "pink", "Dialing Peer", 0)

    elif event_type == "*main.OpenStreamStart":
        track_start_event(stream_states, peer_id)

    elif event_type == "*main.OpenStreamEnd":
        handle_end_event(stream_states, row, "blue", "lightblue", "Opening Stream", 0.15)

    elif event_type == "*main.SendMessageStart":
        track_start_event(message_states, peer_id)

    elif event_type == "*main.SendMessageEnd":
        handle_end_event(message_states, row, "purple", "plum", "Adding Provider", 0.3)

fig.show()
