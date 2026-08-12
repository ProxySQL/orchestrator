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

test("incident model derives actionable, blocked, downtimed, and structural entries", function() {
  const clusters = [{ClusterName: "mysql1:3306", ClusterAlias: "mysql1:3306", CountInstances: 3}];
  const replicationAnalysis = {Details: [{
    Analysis: "DeadMaster",
    AnalyzedInstanceKey: {Hostname: "mysql1", Port: 3306},
    ClusterDetails: {ClusterName: "mysql1:3306"},
    CountReplicas: 2,
    IsDowntimed: false,
    StructureAnalysis: ["ErrantGTIDStructureWarning"],
  }]};
  const blocked = [{
    FailedInstanceKey: {Hostname: "mysql1", Port: 3306},
    Analysis: "DeadMaster",
  }];

  const model = sandbox.buildClustersAnalysisModel(
    clusters,
    replicationAnalysis,
    blocked,
    {DeadMaster: true}
  );

  assert.equal(model.incidentCount, 2);
  assert.equal(model.clusters.length, 1);
  assert.equal(model.clusters[0].topologyPath, "/web/cluster/mysql1:3306?compact=true");
  assert.equal(model.clusters[0].state, "blocked");
  assert.deepEqual(JSON.parse(JSON.stringify(model.clusters[0].entries)), [
    {
      analysis: "DeadMaster",
      instance: "mysql1:3306",
      state: "blocked",
      statusLabel: "Recovery blocked",
      impactLabel: "Affected replicas",
      replicaCount: 2,
      downtimeEndTimestamp: "",
    },
    {
      analysis: "ErrantGTIDStructureWarning",
      instance: "mysql1:3306",
      state: "warning",
      statusLabel: "Structural warning",
      impactLabel: "Participating replicas",
      replicaCount: 2,
      downtimeEndTimestamp: "",
    },
  ]);
});

test("incident model reports a downtimed analysis without mutating API input", function() {
  const entry = {
    Analysis: "DeadMaster",
    AnalyzedInstanceKey: {Hostname: "mysql1", Port: 3306},
    ClusterDetails: {ClusterName: "mysql1:3306"},
    CountReplicas: 2,
    IsDowntimed: true,
    DowntimeEndTimestamp: "2026-08-12 04:00:00",
    StructureAnalysis: [],
  };
  const model = sandbox.buildClustersAnalysisModel(
    [{ClusterName: "mysql1:3306", ClusterAlias: "production", CountInstances: 3}],
    {Details: [entry]},
    [],
    {DeadMaster: true}
  );

  assert.equal(model.clusters[0].alias, "production");
  assert.equal(model.clusters[0].state, "downtimed");
  assert.equal(model.clusters[0].entries[0].statusLabel, "Downtimed");
  assert.equal(model.clusters[0].entries[0].downtimeEndTimestamp, "2026-08-12 04:00:00");
  assert.equal(entry.IsStructureAnalysis, undefined);
  assert.equal(entry.Analysis, "DeadMaster");
});
