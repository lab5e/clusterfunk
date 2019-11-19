var app = new Vue({
    el: '#demoApp',
    data: {
        nodeId: 'bb8727171feccaf0',
        state: 'operational',
        role: 'follower',
        members: [
            {
                nodeId: 'bb8727171feccaf0',
                httpEndpoint: 'http://127.0.0.1:1234/',
                shards: 1000,
            },
            {
                nodeId: '9e9759dc135832f7',
                httpEndpoint: 'http://127.0.0.1:1234/',
                shards: 1000,
            },
            {
                nodeId: 'e065ec51bc239cd5',
                httpEndpoint: 'http://127.0.0.1:1234/',
                shards: 1000,
            }
        ],
        toColor: function (nodeId) {
            // This converts a string into a color. Each charater is xor'ed together to make a
            // single byte and the six lower bits define the hex value. Each value is multiplied up to
            // make 4 discrete values for each r, g, b value.
            v = 0;
            for (var i = 0; i < nodeId.length; i++) {
                v = (v ^ (nodeId.charCodeAt(i)));
            }
            r = ((v & 60) >> 4) << 6;
            g = ((v & 12) >> 2) << 6;
            b = (v & 3) << 6;
            return '#' + ("00" + r.toString(16)).substr(-2) + ("00" + g.toString(16)).substr(-2) + ("00" + b.toString(16)).substr(-2);
        }
    },

});

