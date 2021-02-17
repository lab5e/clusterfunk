/*jslint es6 */
"use strict";

let status = {
    nodeId: '',
    state: '',
    role: '',
    statusHistory: [],
    members: []
};

function showStatus(status) {
    d3.select('#node-color').style('background-color', nodeIdToColor(status.nodeId));
    d3.select('#node-id').text('ID: ' + status.nodeId);
    d3.select('#node-state').text(status.state);
    d3.select('#node-role').text(status.role);
}

function showMemberList(members) {
    d3.select('#member-table').selectAll('tr')
        .data(members)
        .join('tr')
        .html((d) => {
            let backgroundColor = nodeIdToColor(d.nodeId);
            let role = (d.leader ? 'Leader' : 'Follower');
            return `
                <td style="background-color:${backgroundColor}"></td>
                <td>${d.nodeId}</td>
                <td>${role}</td>
                <td>${d.shards}</td>
                <td><a href="${d.httpEndpoint}">Status page</a></td>`;
        })
        .on("mouseover", (d) => setActive(d.nodeId))
        .on("mouseout", (d) => setActive(''));
}

function statusMessage(msg) {
    status.nodeId = msg.nodeId;
    status.state = msg.state;
    status.role = msg.role;
    status.statusHistory.unshift({
        time: new Date(),
        state: msg.state,
        role: msg.role
    });
    if (status.statusHistory.length > 10) {
        status.statusHistory.pop();
    }
    showStatus(status);
    showCluster(status.members);
}

function memberMessage(msg) {
    status.members.forEach((m) => { m.remove = true });
    let urls = [];
    msg.members.forEach((m) => {
        const index = status.members.findIndex((member) => member.nodeId == m.id);
        if (index > -1) {
            status.members[index].httpEndpoint = 'http://' + m.http;
            status.members[index].metricsEndpoint = 'http://' + m.metrics + '/metrics';
            status.members[index].remove = false;
            status.members[index].leader = (m.id == msg.leaderId);
            status.members[index].requests = 0;
            status.members[index].percent = 0;
            urls.push(status.members[index].metricsEndpoint);
            return;
        }
        let newMember = {
            nodeId: m.id,
            httpEndpoint: 'http://' + m.http,
            metricsEndpoint: 'http://' + m.metrics + '/metrics',
            remove: false,
            shards: 0,
            requests: 0,
            percent: 0,
            leader: (m.id == msg.leaderId)
        };
        status.members.push(newMember);
    });

    status.members = status.members.filter((m) => !m.remove);
    showCluster(status.members);
    showMemberList(status.members);
    updateChord();
}

function shardMessage(msg) {
    status.members.forEach(function (m, i) {
        m.shards = msg.shards[m.nodeId];
    });
    showMemberList(status.members);
}

function setWebsocketStatus(msg, active) {
    d3.select('#websocket').text(msg);

    d3.select('#websocket-container')
        .classed('active', active)
        .classed('inactive', !active);
}

function start() {
    let url = new URL('/statusws', window.location.href);
    url.protocol = url.protocol.replace('http', 'ws');
    let ws = new WebSocket(url.href);

    ws.addEventListener('message', (ev) => {
        if (ev.data == null) {
            return;
        }
        let msg = JSON.parse(ev.data);
        if (!msg) {
            return;
        }

        switch (msg.type) {
            case "status":
                statusMessage(msg);
                break;

            case "members":
                memberMessage(msg);
                break;

            case "shards":
                shardMessage(msg);
                break;

            default:
                console.log('Unknown message', msg);
        }
    });

    ws.addEventListener('error', (ev) => {
        console.log('Error on websocket: ', ev)
        setWebsocketStatus('Error', false);
    });

    ws.addEventListener('open', (ev) => {
        setWebsocketStatus('Connected', true);
    });

    ws.addEventListener('close', (ev) => {
        setWebsocketStatus('Disconnected', false);
    });
}

