function recolorNetwork() {
    d3.select('#clusterNetwork')
        .select('#clusterNodes')
        .selectAll('rect')
        .attr("opacity", d => isActive(d.id) ? 1.0 : 0.2);
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