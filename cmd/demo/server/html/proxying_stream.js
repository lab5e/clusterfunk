
/* stream graph for proxying */
function pollMetrics() {
    app.members.forEach(m => {
        if (m.nodeId == app.nodeId) {
            axios.get(m.metricsEndpoint).then(resp => {
                updateProxyData(metricsData(resp.data));
                updateFlowChart(proxyData);
            })
        }
    });
}

window.setInterval(pollMetrics, 2000);

// This array contains a list of changes since last time, ie the difference in count from the last array.
let proxyData = [];
let currentCount = {}
function updateProxyData(metrics) {

    let newItem = metrics.reduce((a, d) => { a[d.destination] = d.count; return a }, {});

    // push the first element
    if (proxyData.length == 0) {
        proxyData.push(newItem);
        currentCount = newItem
        return
    }

    // Calculate difference, if none -- skip it.
    let changes = 0;
    newElement = { time: new Date() }
    for (var prop in newItem) {
        newElement[prop] = newItem[prop] - currentCount[prop]
        changes += newElement[prop]
    }
    if (changes > 0) {
        proxyData.push(newElement);
        currentCount = newItem
    }

    if (proxyData.length > 100) {
        proxyData.shift();
    }
}

function updateFlowChart(data) {

}