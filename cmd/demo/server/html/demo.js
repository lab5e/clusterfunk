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
        toColor: function (nodeId) {
            // This converts a string into a color. Each charater is xor'ed together to make a
            // single byte and the six lower bits define the hex value. Each value is multiplied up to
            // make 4 discrete values for each r, g, b value.
            v = 255;
            for (var i = 0; i < nodeId.length; i++) {
                v = (v ^ (nodeId.charCodeAt(i)));
            }
            r = ((v & 60) >> 4) << 6;
            g = ((v & 12) >> 2) << 6;
            b = (v & 3) << 6;
            return '#' + ("00" + r.toString(16)).substr(-2) + ("00" + g.toString(16)).substr(-2) + ("00" + b.toString(16)).substr(-2);
        },
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
        },

        memberMessage: function (msg) {
            app.members.forEach(function (m, i) {
                m.remove = true;
            });
            msg.members.forEach(function (m, i) {
                const index = app.members.findIndex((member) => member.nodeId == m.id);
                if (index > -1) {
                    app.members[index].httpEndpoint = 'http://' + m.http;
                    app.members[index].metricsEndpoint = 'http://' + m.metrics + '/metrics';
                    app.members[index].remove = false;
                    app.members[index].leader = (m.id == msg.leaderId);
                    return;
                }
                var newMember = {
                    nodeId: m.id,
                    httpEndpoint: 'http://' + m.http,
                    metricsEndpoint: 'http://' + m.metrics + '/metrics',
                    remove: false,
                    shards: 0,
                    leader: (m.id == msg.leaderId)
                };
                app.members.push(newMember);
            });

            app.members = app.members.filter(function (m, i) {
                return !m.remove;
            });
        },
        shardMessage: function (msg) {
            app.members.forEach(function (m, i) {
                m.shards = msg.shards[m.nodeId];
            });
        },
        /* this is unused ATM */
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
        }
    }
});

