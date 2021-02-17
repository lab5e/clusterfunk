/*jslint es6 */
"use strict";

let clusterRequestState = {
    requestData: [],
    lastRequestCount: 0,
    maxRequests: 50,
    margins: { top: 5, left: 40, bottom: 35, right: 5 }
};

function updateClusterRequestCount(metrics) {
    let sum = 0;
    metrics.forEach((r) => r.forEach((d) => {
        // Add just the requests that are handled locally. Proxied requests
        // are handled locally elsewhere.
        if (d.node === d.destination) {
            sum += d.count;
        }
    }));
    let count = Math.max(0, (sum - clusterRequestState.lastRequestCount) / (clusterRefreshRate / 1000));
    clusterRequestState.lastRequestCount = sum;


    if (clusterRequestState.requestData.length == 0) {
        for (let i = 0; i < clusterRequestState.maxRequests; i++) {
            clusterRequestState.requestData.push({ value: i % 2 });
        }
        clusterRequestState.lastRequestCount = sum;
        return;
    }
    clusterRequestState.requestData.push({ time: new Date(), value: count });
    clusterRequestState.requestData.shift();
    updateClusterRequestChart(clusterRequestState.requestData);
}

function updateClusterRequestChart(data) {
    const container = d3.select('#cluster-request-container').node().getBoundingClientRect();
    const requestSize = { width: container.width, height: container.height };
    const svg = d3.select('#cluster-request');

    const xScale = d3.scaleTime()
        .domain(d3.extent(data, d => d.time))
        .range([clusterRequestState.margins.left, requestSize.width - clusterRequestState.margins.right]);

    const xAxis = d3.axisBottom(xScale).ticks(5, d3.timeFormat("%H:%M:%S"));

    let dom = d3.extent(data, d => d.value)
    dom[0] = 0;
    dom[1] = Math.max(dom[1], 10);

    const yScale = d3.scaleLinear()
        .domain(dom)
        .range([requestSize.height - clusterRequestState.margins.bottom, clusterRequestState.margins.top]);


    const yAxis = d3.axisLeft(yScale).ticks(5);

    const line = d3.line()
        .defined(d => !isNaN(d.time))
        .x(d => xScale(d.time))
        .y(d => yScale(d.value))

    const elems = svg.select('g.plot')
        .selectAll('path')
        .data([data])
        .join('path')
        .attr('fill', 'none')
        .attr('stroke', 'black')
        .attr('stroke-width', 2)
        .attr('d', line);

    elems.attr('d', line);

    svg.select('g.xaxis')
        .attr('transform', `translate(0, ${requestSize.height - clusterRequestState.margins.bottom})`)
        .call(xAxis);

    svg.select('g.yaxis')
        .attr('transform', `translate(${clusterRequestState.margins.left}, 0)`)
        .call(yAxis);
}