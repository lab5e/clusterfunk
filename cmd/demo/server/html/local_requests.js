/*jslint es6 */
"use strict";

let requestData = [];
let lastRequestCount = 0;
const maxRequests = 50;
const requestMargins = { top: 5, left: 30, right: 5, bottom: 30 };

function updateRequestCount(metrics) {
    let sum = 0;
    metrics.forEach((d) => {
        sum += d.count;
    });
    if (requestData.length == 0) {
        for (let i = 0; i < maxRequests; i++) {
            requestData.push({ value: i % 2 });
        }
        lastRequestCount = sum;
        return;
    }
    requestData.push({ time: new Date(), value: (sum - lastRequestCount) });
    requestData.shift();
    lastRequestCount = sum;
    updateRequestChart(requestData);
}

function updateRequestChart(data) {
    const container = d3.select('#node-request-container').node().getBoundingClientRect();
    const requestSize = { width: container.width, height: container.height };
    const svg = d3.select('#node-request');

    const xScale = d3.scaleTime()
        .domain(d3.extent(data, d => d.time))
        .range([requestMargins.left, requestSize.width - requestMargins.right]);

    const xAxis = d3.axisBottom(xScale).ticks(5, d3.timeFormat("%H:%M:%S"));

    const yScale = d3.scaleLinear()
        .domain(d3.extent(data, d => d.value))
        .range([requestSize.height - requestMargins.bottom, requestMargins.top]);


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
        .attr('stroke', nodeIdToColor(status.nodeId))
        .attr('stroke-width', 2)
        .attr('d', line);

    elems.attr('d', line);

    svg.select('g.xaxis')
        .attr('transform', `translate(0, ${requestSize.height - requestMargins.bottom})`)
        .call(xAxis);

    svg.select('g.yaxis')
        .attr('transform', `translate(${requestMargins.left}, 0)`)
        .call(yAxis);
}