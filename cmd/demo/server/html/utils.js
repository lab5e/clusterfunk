/*jslint es6 */
"use strict";

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

let activeNodeId = '';

function setActive(nodeId) {
    activeNodeId = nodeId;

    d3.select('#cluster-network')
        .select('#cluster-node')
        .selectAll('rect')
        .attr("opacity", d => isActive(d.id) ? 1.0 : 0.2);

    d3.select('#cluster-chord')
        .selectAll('g.group')
        .selectAll('path')
        .attr('opacity', d => isActive(chordData.indexToName.get(d.index)) ? 1.0 : 0.2);

    d3.select('#cluster-chord')
        .selectAll('path.links')
        .attr('opacity', d => isActive(chordData.indexToName.get(d.target.index)) ? 1.0 : 0.2);

    d3.select('#node-proxy')
        .select('g.plot')
        .selectAll('path').attr('opacity', (d, i) => isActive(streamKeys[i]) ? 1.0 : 0.2);
}

function isActive(nodeId) {
    if (activeNodeId == '') {
        return true;
    }
    return (nodeId == activeNodeId);
}