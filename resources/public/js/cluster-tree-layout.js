(function(root) {
  function layout(d3, topologyRoot, treeLayout, horizontalSpacing) {
    var hierarchyRoot = d3.hierarchy(topologyRoot, function(node) { return node.children; });
    treeLayout(hierarchyRoot);
    var nodes = [];
    hierarchyRoot.each(function(hierarchyNode) {
      var node = hierarchyNode.data;
      node.x = hierarchyNode.x;
      node.y = node.isAnchor
        ? node.virtualDepth * horizontalSpacing - horizontalSpacing / 2
        : node.virtualDepth * horizontalSpacing;
      nodes.push(node);
    });
    return {
      nodes: nodes.reverse(),
      links: hierarchyRoot.links().map(function(link) {
        return {source: link.source.data, target: link.target.data};
      })
    };
  }
  root.OrchestratorTreeLayout = {layout: layout};
})(typeof window === 'undefined' ? globalThis : window);
