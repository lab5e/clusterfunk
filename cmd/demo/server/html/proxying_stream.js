
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

window.setInterval(pollMetrics, 1000);
const flowChartWidth = 600;
const flowChartHeight = 200;
const maxSampleCount = 50;


// This array contains a list of changes since last time, ie the difference in count from the last array.
let proxyData = [];
let currentCount = {}
function updateProxyData(metrics) {

    let newItem = metrics.reduce((a, d) => { a[d.destination] = d.count; return a }, {});

    // push the first element
    if (proxyData.length == 0) {
        for (var p in newItem) {
            currentCount[p] = newItem[p];
            newItem[p] = 0;
        }
        proxyData.push(newItem);
        return
    }

    // Calculate difference, if none -- skip it.
    let changes = 0;
    let newElement = {};

    for (var prop in currentCount) {
        if (isNaN(newItem[prop])) {
            newItem[prop] = 0;
        }
        if (isNaN(currentCount[prop])) {
            currentCount[prop] = 0;
        }
        newElement[prop] = newItem[prop] - currentCount[prop]
        changes += newElement[prop]
    }
    proxyData.push(newElement);
    currentCount = newItem;
    if (proxyData.length > maxSampleCount) {
        proxyData.shift();
    }

}

function dataKeys(data) {
    let ret = new Map()
    data.forEach(d => {
        for (var p in d) {
            ret.set(p, 0);
        }
    });
    r = []
    ret.forEach((v, k) => r.push(k));
    return r;
}

function domainY(data) {
    let ret = [100, -100];
    data.forEach(d => {
        d.forEach(i => {
            ret[0] = Math.min(ret[0], i[0])
            ret[0] = Math.min(ret[0], i[1])
            ret[1] = Math.max(ret[1], i[1])
            ret[1] = Math.max(ret[1], i[0])
        })
    })
    return ret
}

function updateFlowChart(data) {
    let keys = dataKeys(data);
    let stack = d3.stack().keys(keys).value((d, key) => {
        if (isNaN(d[key])) { return 0; }
        return d[key];
    }).order(d3.stackOrderDescending) // .offset(d3.stackOffsetWiggle);
    let series = stack(data);

    let xScale = d3.scaleLinear()
        .domain([0, maxSampleCount - 1])
        .range([0, flowChartWidth]);

    let yScale = d3.scaleLinear()
        .domain(domainY(series))
        .range([flowChartHeight, 0]);

    let area = d3.area()
        //.curve(d3.curveNatural)
        .x((d, i) => xScale(i))
        .y0((d) => yScale(d[0]))
        .y1((d) => yScale(d[1]));

    const svg = d3.select('#proxyStream').select('g.plot');
    svg.selectAll("path")
        .data(series)
        .join("path")
        .transition()
        .style("fill", (d, i) => nodeIdToColor(keys[i]))
        .attr("d", area);

    //svg.append("g")
    //    .call(xAxis);
}



