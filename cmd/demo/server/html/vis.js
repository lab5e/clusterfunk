const clusterWidth = 600;
const clusterHeight = 500;
const rectWidth = 150;
const rectHeight = 25;
const nodeColors = d3.scaleOrdinal(d3.schemeCategory10)
let colorMap = [];
let nodeCounter = 0;

function nodeIdToColor(nodeId) {
    let col = colorMap[nodeId];
    if (!col) {
        nodeCounter++
        colorMap[nodeId] = nodeCounter
    }
    return nodeColors(colorMap[nodeId]);
}

function showCluster(members) {
    let leaderId = "";
    let nodes = [];
    members.forEach(d => {
        if (d.leader) {
            leaderId = d.nodeId;
        }
        nodes.push(Object.create({ id: d.nodeId, leader: d.leader }));
    });
    let links = [];
    members.forEach(d => {
        if (d.leader) {
            return;
        }
        links.push(Object.create({ source: d.nodeId, target: leaderId }));
    });

    const simulation = d3.forceSimulation()
        .nodes(nodes)
        .force("charge", d3.forceManyBody().strength(-1600))
        .force("center", d3.forceCenter(clusterWidth / 2, clusterHeight / 2))
        .force("link", d3.forceLink(links).id(d => d.id).distance(175))

    const svg = d3.select('#clusterNetwork');

    let rects = svg.select("#clusterNodes").selectAll("rect")
        .data(nodes)
        .enter().append("g").attr("class", "box");

    rects.append("rect")
        .transition().duration(100)
        .attr("stroke", "black")
        .attr("rx", 10)
        .attr("width", rectWidth)
        .attr("height", rectHeight)
        .attr("fill", d => nodeIdToColor(d.id))
    rects.append("text").text(d => d.id).attr("text-anchor", "middle");

    svg.select("#clusterNodes").selectAll("rect").data(nodes).exit().remove();
    svg.select("#clusterNodes").selectAll("text").data(nodes).exit().remove();

    svg.select("#clusterLinks").selectAll("line.network")
        .data(links)
        .enter()
        .append("line")
        .transition().duration(100)
        .attr('class', 'network')
        .attr("stroke", "black")
        .attr("stroke-width", 1);

    svg.select("#clusterLinks").selectAll("line").data(links).exit().remove();


    simulation.on("tick", () => {
        d3.selectAll("line.network")
            .attr("x1", d => d.source.x)
            .attr("y1", d => d.source.y)
            .attr("x2", d => d.target.x)
            .attr("y2", d => d.target.y);

        d3.selectAll("rect")
            .attr("x", d => d.x - (rectWidth / 2))
            .attr("y", d => d.y - (rectHeight / 2));

        d3.selectAll("text")
            .attr("x", d => d.x)
            .attr("y", d => (d.y + 5));

    });

    //invalidation.then(() => simulation.stop());
}

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

/* stream graph for proxying */
function pollMetrics() {
    app.members.forEach(m => {
        if (m.nodeId == app.nodeId) {
            axios.get(m.metricsEndpoint).then(resp => {
                updateProxyData(metricsData(resp.data));
            })
        }
    });
}

window.setInterval(pollMetrics, 1000);

let proxyData = [];

function updateProxyData(metrics) {
    /*    console.log(metrics);
        const svg = d3.select('#proxyStream')
        const path = svg.selectAll('path')
        .data(proxyData)
        .join('path')
        .attr('d', area)
        .attr('fill', (d) => nodeIdToColor(d.targetId))*/
}

