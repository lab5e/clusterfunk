
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

//window.setInterval(pollMetrics, 2000);

// This array contains a list of changes since last time, ie the difference in count from the last array.
let proxyData = [];
let currentCount = {}
function updateProxyData(metrics) {

    let newItem = metrics.reduce((a, d) => { a[d.destination] = d.count; return a }, {});

    // push the first element
    if (proxyData.length == 0) {
        newItem.time = newDate();
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

    console.log(JSON.stringify(proxyData))
}




/*
[{"time":"2019-11-25T23:34:09.184Z","18bfdaa96cc77b24":5,"4ccff194d95142c1":7,"a468d5959ff93323":8}]
[{"time":"2019-11-25T23:34:09.184Z","18bfdaa96cc77b24":5,"4ccff194d95142c1":7,"a468d5959ff93323":8},
 {"time":"2019-11-25T23:34:11.186Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":6,"a468d5959ff93323":6}]
[{"time":"2019-11-25T23:34:09.184Z","18bfdaa96cc77b24":5,"4ccff194d95142c1":7,"a468d5959ff93323":8},
 {"time":"2019-11-25T23:34:11.186Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":6,"a468d5959ff93323":6},
 {"time":"2019-11-25T23:34:13.183Z","18bfdaa96cc77b24":4,"4ccff194d95142c1":8,"a468d5959ff93323":8}]
[{"time":"2019-11-25T23:34:09.184Z","18bfdaa96cc77b24":5,"4ccff194d95142c1":7,"a468d5959ff93323":8},
 {"time":"2019-11-25T23:34:11.186Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":6,"a468d5959ff93323":6},
 {"time":"2019-11-25T23:34:13.183Z","18bfdaa96cc77b24":4,"4ccff194d95142c1":8,"a468d5959ff93323":8},
 {"time":"2019-11-25T23:34:15.187Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":8,"a468d5959ff93323":4}]
[{"time":"2019-11-25T23:34:09.184Z","18bfdaa96cc77b24":5,"4ccff194d95142c1":7,"a468d5959ff93323":8},
 {"time":"2019-11-25T23:34:11.186Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":6,"a468d5959ff93323":6},
 {"time":"2019-11-25T23:34:13.183Z","18bfdaa96cc77b24":4,"4ccff194d95142c1":8,"a468d5959ff93323":8},
 {"time":"2019-11-25T23:34:15.187Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":8,"a468d5959ff93323":4},
 {"time":"2019-11-25T23:34:17.189Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":7,"a468d5959ff93323":6}]
[{"time":"2019-11-25T23:34:09.184Z","18bfdaa96cc77b24":5,"4ccff194d95142c1":7,"a468d5959ff93323":8},
 {"time":"2019-11-25T23:34:11.186Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":6,"a468d5959ff93323":6},
 {"time":"2019-11-25T23:34:13.183Z","18bfdaa96cc77b24":4,"4ccff194d95142c1":8,"a468d5959ff93323":8},
 {"time":"2019-11-25T23:34:15.187Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":8,"a468d5959ff93323":4},
 {"time":"2019-11-25T23:34:17.189Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":7,"a468d5959ff93323":6},
 {"time":"2019-11-25T23:34:19.187Z","18bfdaa96cc77b24":6,"4ccff194d95142c1":9,"a468d5959ff93323":4}]
[{"time":"2019-11-25T23:34:09.184Z","18bfdaa96cc77b24":5,"4ccff194d95142c1":7,"a468d5959ff93323":8},
 {"time":"2019-11-25T23:34:11.186Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":6,"a468d5959ff93323":6},
 {"time":"2019-11-25T23:34:13.183Z","18bfdaa96cc77b24":4,"4ccff194d95142c1":8,"a468d5959ff93323":8},
 {"time":"2019-11-25T23:34:15.187Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":8,"a468d5959ff93323":4},
 {"time":"2019-11-25T23:34:17.189Z","18bfdaa96cc77b24":7,"4ccff194d95142c1":7,"a468d5959ff93323":6},
 {"time":"2019-11-25T23:34:19.187Z","18bfdaa96cc77b24":6,"4ccff194d95142c1":9,"a468d5959ff93323":4},
 {"time":"2019-11-25T23:34:21.189Z","18bfdaa96cc77b24":6,"4ccff194d95142c1":7,"a468d5959ff93323":6}]
*/