/*jslint es6 */
"use strict";

let status = {
    nodeId: '',
    state: '',
    role: '',
    statusHistory: [],
    members: []
};

const nodeColors = d3.scaleOrdinal(d3.schemeCategory10);
let colorMap = [];
let nodeCounter = 0;

function nodeIdToColor(nodeId) {
    let col = colorMap[nodeId];
    if (!col) {
        nodeCounter++;
        colorMap[nodeId] = nodeCounter;
    }
    return nodeColors(colorMap[nodeId]);
}

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
                <td><a href="${d.httpEndpoint}">Status page</a></td>
                <td><a href="${d.metricsEndpoint}">Metrics</a></td>`;
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

/*
var app = new Vue({
    el: '#demoApp',
    data: {
        nodeId: 'x',
        state: 'x',
        role: 'x',
        websocketStatus: 'x',
        members: [],
        statusHistory: [],
        metrics: []
    },
    mounted: function () {
        // Set up websocket
        var url = new URL('/statusws', window.location.href);
        url.protocol = url.protocol.replace('http', 'ws');
        var ws = new WebSocket(url.href);

        ws.onmessage = function (ev) {
            if (ev.data == null) {
                return;
            }
            var msg = JSON.parse(ev.data);
            if (!msg) {
                return;
            }
            switch (msg.type) {
                case "status":
                    app.statusMessage(msg);
                    break;

                case "members":
                    app.memberMessage(msg);

                    break;

                case "shards":
                    app.shardMessage(msg);
                    break;
                default:
                    console.log('Unknown message', msg);
            }
        };
        ws.onerror = function (s, ev) {
            console.log('Error on websocket: ', ev)
            app.websocketStatus = 'error';
        };
        ws.onopen = function (s, ev) {
            app.websocketStatus = 'connected';
        };
        ws.onclose = function (s, ev) {
            app.websocketStatus = 'disconnected';
        };

    },
    methods: {
        statusMessage: function (msg) {
            app.nodeId = msg.nodeId;
            app.state = msg.state;
            app.role = msg.role;
            app.statusHistory.unshift({
                time: new Date(),
                state: msg.state,
                role: msg.role
            });
            if (app.statusHistory.length > 10) {
                app.statusHistory.pop();
            }
            showCluster(app.members);
        },

        memberMessage: function (msg) {
            app.members.forEach(function (m, i) {
                m.remove = true;
            });
            let urls = [];
            msg.members.forEach(function (m, i) {
                const index = app.members.findIndex((member) => member.nodeId == m.id);
                if (index > -1) {
                    app.members[index].httpEndpoint = 'http://' + m.http;
                    app.members[index].metricsEndpoint = 'http://' + m.metrics + '/metrics';
                    app.members[index].remove = false;
                    app.members[index].leader = (m.id == msg.leaderId);
                    app.members[index].requests = 0;
                    app.members[index].percent = 0;
                    urls.push(app.members[index].metricsEndpoint);
                    return;
                }
                var newMember = {
                    nodeId: m.id,
                    httpEndpoint: 'http://' + m.http,
                    metricsEndpoint: 'http://' + m.metrics + '/metrics',
                    remove: false,
                    shards: 0,
                    requests: 0,
                    percent: 0,
                    leader: (m.id == msg.leaderId)
                };
                app.members.push(newMember);
            });

            app.members = app.members.filter(function (m, i) {
                return !m.remove;
            });
            showCluster(app.members);
            updateChord();

        },
        shardMessage: function (msg) {
            app.members.forEach(function (m, i) {
                m.shards = msg.shards[m.nodeId];
            });
        },
        /* this is unused ATM */
/*
loadMetrics: function () {
const index = app.members.findIndex((member) => member.nodeId == app.nodeId);
if (index > -1) {
app.showMetrics(app.members[index].metricsEndpoint);
}
},
showMetrics: function (url) {
axios.get(url)
.then(function (response) {
    app.metrics = [];
    response.data.split('\n').forEach(function (line, i) {
        if (line.substr(0, 1) == '#') {
            return;
        }
        nameValue = line.split(' ');
        app.metrics.push({
            name: nameValue[0],
            value: nameValue[1]
        });
    });
})
.catch(function (error) {
    console.log('Error reading metrics: ', error);
});
},
toColor: function (nodeId) {
return nodeIdToColor(nodeId);
}
}
});
*/