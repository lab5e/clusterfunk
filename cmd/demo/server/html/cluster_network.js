const clusterWidth = 400;
const clusterHeight = 400;
const rectWidth = 25;
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
        .force("charge", d3.forceManyBody().strength(-800))
        .force("center", d3.forceCenter(clusterWidth / 2, clusterHeight / 2))
        .force("link", d3.forceLink(links).id(d => d.id).distance(75))

    const svg = d3.select('#clusterNetwork');

    let rects = svg.select("#clusterNodes")
        .selectAll("rect")
        .data(nodes)
        .enter().append("g").attr("class", "box")
        .on("mouseover", (d) => setActive(d.id))
        .on("mouseout", (d) => setActive(''));

    rects.append("rect")
        .transition().duration(100)
        .attr("rx", 5)
        .attr("width", rectWidth)
        .attr("height", rectHeight)
        .attr("fill", d => nodeIdToColor(d.id))


    svg.select("#clusterNodes")
        .selectAll("rect")
        .data(nodes)
        .exit()
        .remove();

    svg.select("#clusterNodes")
        .selectAll("rect")
        .data(nodes)
        .attr("stroke", d => d.id == app.nodeId ? 'black' : 'silver')
        .attr("stroke-width", d => d.id == app.nodeId ? '2px' : '1px')

    svg.select("#clusterLinks").selectAll("line.network")
        .data(links)
        .enter()
        .append("line")
        .transition().duration(100)
        .attr('class', 'network')
        .attr("stroke", "silver")
        .attr("stroke-width", 1);

    svg.select("#clusterLinks")
        .selectAll("line")
        .data(links)
        .exit()
        .remove();


    simulation.on("tick", () => {
        d3.selectAll("line.network")
            .attr("x1", d => d.source.x)
            .attr("y1", d => d.source.y)
            .attr("x2", d => d.target.x)
            .attr("y2", d => d.target.y);

        d3.selectAll("rect")
            .attr("x", d => d.x - (rectWidth / 2))
            .attr("y", d => d.y - (rectHeight / 2));

    });
}

