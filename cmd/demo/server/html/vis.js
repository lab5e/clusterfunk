const clusterWidth = 600;
const clusterHeight = 500;
const rectWidth = 150;
const rectHeight = 25;
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

    console.log('Nodes', nodes, nodes.length);
    console.log('Links', links, links.length);

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
        .attr("fill", d => app.toColor(d.id))
    rects.append("text").text(d => d.id).attr("text-anchor", "middle");

    svg.select("#clusterNodes").selectAll("rect").data(nodes).exit().remove();
    svg.select("#clusterNodes").selectAll("text").data(nodes).exit().remove();

    svg.select("#clusterLinks").selectAll("line")
        .data(links)
        .enter()
        .append("line")
        .transition().duration(100)
        .attr("stroke", "black")
        .attr("stroke-width", 1);

    svg.select("#clusterLinks").selectAll("line").data(links).exit().remove();


    simulation.on("tick", () => {
        d3.selectAll("line")
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