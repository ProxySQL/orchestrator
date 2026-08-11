const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const sourcePath = path.resolve(__dirname, "../../../resources/public/js/clusters-analysis.js");
const source = fs.readFileSync(sourcePath, "utf8");
const sandbox = {
  document: {},
  $: function() {
    return {ready: function() {}};
  },
};

vm.runInNewContext(source, sandbox, {filename: sourcePath});

test("analysis links use the cluster-name route when the alias is the default", function() {
  const cluster = {
    ClusterName: "mysql1:3306",
    ClusterAlias: "mysql1:3306",
  };

  assert.equal(sandbox.clusterAnalysisTopologyPath(cluster, false), "/web/cluster/mysql1:3306");
  assert.equal(sandbox.clusterAnalysisTopologyPath(cluster, true), "/web/cluster/mysql1:3306?compact=true");
});

test("analysis links use an encoded route for a distinct alias", function() {
  const cluster = {
    ClusterName: "mysql1:3306",
    ClusterAlias: "production east",
  };

  assert.equal(sandbox.clusterAnalysisTopologyPath(cluster, false), "/web/cluster/alias/production%20east");
  assert.equal(sandbox.clusterAnalysisTopologyPath(cluster, true), "/web/cluster/alias/production%20east?compact=true");
});
