
function retrieveAllMetrics(urls) {
    let promises = [];
    urls.forEach(url => promises.push(axios.get(url)));
    return Promise.all(promises).then(datasets => {
        let allMetrics = [];
        datasets.forEach(set => {
            allMetrics.push(metricsData(set.data))
        });
        return allMetrics;
    })
}

// Massage metrics data into a passable proxy struct. The Prometheus metrics are
// JSON-like at best.
function metricsData(metrics) {
    let ret = [];
    metrics.split('\n').forEach((line, i) => {
        if (line.substr(0, 1) == '#') {
            return;
        }
        let nameValue = line.split(' ');
        // Massage the value to make a JSON object
        if (nameValue.length == 2 && nameValue[0].substr(0, 19) == 'cf_cluster_requests') {
            let vals = nameValue[0].substr(19)
            let o = JSON.parse(vals
                .replace('destination=', '"destination":')
                .replace('method=', '"method":')
                .replace('node=', '"node":'))
            o.count = +nameValue[1];
            ret.push(o);
        }
    })
    return ret
}

let matrixIndex = 0;

let chordData = {
    matrix: [],
    indexToName: new Map(),
    nameToIndex: new Map()
}

function addNodeToMatrix(nodeId) {
    // Add element to matrix
    chordData.matrix.forEach(d => d.push(0))
    let newElement = [];
    for (let i = 0; i < chordData.matrix.length + 1; i++) {
        newElement.push(0);
    }
    chordData.matrix.push(newElement);
    chordData.nameToIndex.set('' + nodeId, matrixIndex);
    chordData.indexToName.set(matrixIndex, '' + nodeId);
    matrixIndex++

}
// Visualize proxy information
function showProxying(proxyData) {
    // We have a list of metrics for all nodes. Build the matrix.
    // build the list of names and mappings
    proxyData.forEach(line => {
        line.forEach(d => {
            if (!chordData.nameToIndex.has(d.node)) {
                addNodeToMatrix(d.node)
            }
            if (!chordData.nameToIndex.has(d.destination)) {
                addNodeToMatrix(d.destination)
            }
        });
    });
    proxyData.forEach(line => {
        line.forEach(d => {
            let i1 = chordData.nameToIndex.get(d.destination);
            let i2 = chordData.nameToIndex.get(d.node);
            chordData.matrix[i1][i2] = d.count;
        })
    });

    const innerRadius = 280;
    const outerRadius = 290;

    d3.select('#proxyingChart').select('g.contents').remove();

    d3.select('#proxyingChart')
        .append('g')
        .attr('class', 'contents')
        .attr('transform', 'translate(300,300)');

    let svg = d3.select('#proxyingChart').select('g.contents');

    const arc = d3.arc()
        .innerRadius(innerRadius)
        .outerRadius(outerRadius);

    const chord = d3.chord()
        .padAngle(0.05)
        .sortSubgroups(d3.descending)
        (chordData.matrix);

    // Groups in innner part of circle
    const chordGroup = svg.selectAll('g.group')
        .data(chord.groups)
        .join('g').attr('class', 'group');

    chordGroup.append("path")
        .style("fill", d => nodeIdToColor(chordData.indexToName.get(d.index)))
        .style("stroke", d => d3.rgb(nodeIdToColor(chordData.indexToName.get(d.index))).darker())
        .attr("d", arc)

    // links between groups
    const ribbon = d3.ribbon().radius(innerRadius)
    svg.append('g')
        .attr("fill-opacity", 0.5)
        .attr('class', 'links')
        .selectAll('path.links') //.id(d => d.source.index + '-' + d.target.index)
        .data(chord)
        .join('path')
        .attr('class', 'links')
        .attr("d", ribbon)
        .style("fill", (d) => nodeIdToColor(chordData.indexToName.get(d.target.index)))
        .style("stroke", d => d3.rgb(nodeIdToColor(chordData.indexToName.get(d.target.index))).darker());
}
