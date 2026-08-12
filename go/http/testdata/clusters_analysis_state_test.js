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

function renderAdapterWithResponses(clusters, replicationAnalysis, blockedRecoveries) {
  let ready;
  const rendered = {};
  const adapterSandbox = {
    document: {},
    addAlert: function() {},
    appUrl: function(path) { return path; },
    hideLoader: function() {},
    interestingAnalysis: {DeadMaster: true},
    isAuthorizedForAction: function() { return false; },
    showLoader: function() {},
  };
  function jquery(target) {
    if (target === adapterSandbox.document) {
      return {ready: function(callback) { ready = callback; }};
    }
    return {
      hide: function() { rendered.loadingHidden = true; },
      html: function(markup) { rendered.markup = markup; },
      text: function(summary) { rendered.summary = summary; },
    };
  }
  jquery.get = function() {
    return {};
  };
  jquery.when = function() {
    return {
      done: function(callback) {
        callback(clusters, replicationAnalysis, blockedRecoveries);
        return {fail: function() {}};
      },
    };
  };
  adapterSandbox.$ = jquery;

  vm.runInNewContext(source, adapterSandbox, {filename: sourcePath});
  ready();
  return rendered;
}

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

test("incident model derives blocked and structural entries", function() {
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

test("incident model derives an actionable entry", function() {
  const model = sandbox.buildClustersAnalysisModel(
    [{ClusterName: "mysql1:3306", ClusterAlias: "mysql1:3306", CountInstances: 3}],
    {Details: [{
      Analysis: "DeadMaster",
      AnalyzedInstanceKey: {Hostname: "mysql1", Port: 3306},
      ClusterDetails: {ClusterName: "mysql1:3306"},
      CountReplicas: 2,
      IsDowntimed: false,
      StructureAnalysis: [],
    }]},
    [],
    {DeadMaster: true}
  );

  assert.deepEqual(JSON.parse(JSON.stringify(model.clusters[0].entries)), [{
    analysis: "DeadMaster",
    instance: "mysql1:3306",
    state: "actionable",
    statusLabel: "Requires attention",
    impactLabel: "Affected replicas",
    replicaCount: 2,
    downtimeEndTimestamp: "",
  }]);
});

test("incident model sorts entries by state, instance, and analysis", function() {
  const clusterName = "mysql1:3306";
  function entry(analysis, hostname, isDowntimed, structureAnalysis) {
    return {
      Analysis: analysis,
      AnalyzedInstanceKey: {Hostname: hostname, Port: 3306},
      ClusterDetails: {ClusterName: clusterName},
      CountReplicas: 2,
      IsDowntimed: isDowntimed,
      StructureAnalysis: structureAnalysis || [],
    };
  }
  const model = sandbox.buildClustersAnalysisModel(
    [{ClusterName: clusterName, ClusterAlias: clusterName, CountInstances: 6}],
    {Details: [
      entry("DeadMaster", "mysql-z", true),
      entry("NoProblem", "mysql-c", false, ["ErrantGTIDStructureWarning"]),
      entry("DeadMaster", "mysql-b", false),
      entry("DeadMaster", "mysql-a", false),
      entry("DeadMaster", "mysql-blocked", false),
      entry("DeadCoMaster", "mysql-a", false),
    ]},
    [{
      FailedInstanceKey: {Hostname: "mysql-blocked", Port: 3306},
      Analysis: "DeadMaster",
    }],
    {DeadCoMaster: true, DeadMaster: true}
  );

  assert.deepEqual(
    JSON.parse(JSON.stringify(model.clusters[0].entries.map(function(item) {
      return {state: item.state, instance: item.instance, analysis: item.analysis};
    }))),
    [
      {state: "blocked", instance: "mysql-blocked:3306", analysis: "DeadMaster"},
      {state: "actionable", instance: "mysql-a:3306", analysis: "DeadCoMaster"},
      {state: "actionable", instance: "mysql-a:3306", analysis: "DeadMaster"},
      {state: "actionable", instance: "mysql-b:3306", analysis: "DeadMaster"},
      {state: "warning", instance: "mysql-c:3306", analysis: "ErrantGTIDStructureWarning"},
      {state: "downtimed", instance: "mysql-z:3306", analysis: "DeadMaster"},
    ]
  );
});

test("incident model tracks an unmatched structural entry", function() {
  const model = sandbox.buildClustersAnalysisModel(
    [],
    {Details: [{
      Analysis: "NoProblem",
      AnalyzedInstanceKey: {Hostname: "mysql1", Port: 3306},
      ClusterDetails: {ClusterName: "mysql1:3306"},
      CountReplicas: 2,
      IsDowntimed: false,
      StructureAnalysis: ["ErrantGTIDStructureWarning"],
    }]},
    [],
    {}
  );

  assert.equal(model.unmatchedEntryCount, 1);
  assert.equal(model.incidentCount, 0);
  assert.equal(model.clusters.length, 0);
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

test("incident markup is semantic, escaped, and contains one clear topology action", function() {
  const html = sandbox.renderClustersAnalysisMarkup({incidentCount: 1, clusters: [{
    clusterName: "mysql1:3306",
    displayName: "<mysql1>",
    alias: "",
    topologyPath: "/web/cluster/mysql1:3306?compact=true",
    countInstances: 3,
    allDowntimed: false,
    state: "actionable",
    entries: [{
      analysis: "DeadMaster",
      instance: "mysql1:3306",
      state: "actionable",
      statusLabel: "Requires attention",
      impactLabel: "Affected replicas",
      replicaCount: 2,
      downtimeEndTimestamp: "",
    }],
  }]});

  assert.match(html, /<article[^>]+data-cluster-name="mysql1:3306"/);
  assert.match(html, /&lt;mysql1&gt;/);
  assert.doesNotMatch(html, /<mysql1>/);
  assert.match(html, /DeadMaster/);
  assert.match(html, /Affected replicas/);
  assert.match(html, /href="\/web\/cluster\/mysql1:3306\?compact=true"/);
  assert.equal((html.match(/Open topology/g) || []).length, 1);
  assert.doesNotMatch(html, /popover|popover-title|popover-content/);
});

test("empty and unavailable states cannot be confused", function() {
  const empty = sandbox.renderClustersAnalysisEmptyState();
  const unavailable = sandbox.renderClustersAnalysisUnavailableState();

  assert.match(empty, /No active failover incidents/);
  assert.doesNotMatch(empty, /DeadMaster|interestingAnalysis/);
  assert.match(unavailable, /Failure analysis is temporarily unavailable/);
  assert.match(unavailable, /Reload page/);
});

test("document adapter requests each incident source as JSON", function() {
  let ready;
  const requests = [];
  const adapterSandbox = {
    document: {},
    addAlert: function() {},
    appUrl: function(path) { return path; },
    hideLoader: function() {},
    interestingAnalysis: {},
    isAuthorizedForAction: function() { return false; },
    showLoader: function() {},
  };
  function jquery(target) {
    if (target === adapterSandbox.document) {
      return {ready: function(callback) { ready = callback; }};
    }
    return {hide: function() {}, html: function() {}, text: function() {}};
  }
  jquery.get = function() {
    requests.push(Array.from(arguments));
    return {};
  };
  jquery.when = function() {
    return {
      done: function() {
        return {fail: function() {}};
      },
    };
  };
  adapterSandbox.$ = jquery;

  vm.runInNewContext(source, adapterSandbox, {filename: sourcePath});
  ready();

  assert.deepEqual(
    JSON.parse(JSON.stringify(requests.slice(0, 3).map(function(request) {
      return [request[0], request[3]];
    }))),
    [
      ["/api/clusters-info", "json"],
      ["/api/replication-analysis", "json"],
      ["/api/blocked-recoveries", "json"],
    ]
  );
});

test("blocked-recovery alerts escape API-derived text and audit URLs", function() {
  let ready;
  let alert;
  const adapterSandbox = {
    document: {},
    addAlert: function(html) { alert = html; },
    appUrl: function(path) { return path; },
    getInstanceTitle: function(hostname, port) { return hostname + ":" + port; },
    hideLoader: function() {},
    interestingAnalysis: {},
    isAuthorizedForAction: function() { return false; },
    showLoader: function() {},
  };
  function jquery(target) {
    if (target === adapterSandbox.document) {
      return {ready: function(callback) { ready = callback; }};
    }
    return {hide: function() {}, html: function() {}, text: function() {}};
  }
  jquery.get = function(url, callback) {
    if (typeof callback == "function") {
      callback([{
        Analysis: "<script>",
        FailedInstanceKey: {Hostname: "<host>", Port: "3306\" onclick=\"bad"},
        BlockingRecoveryId: "recovery?x=\"bad",
      }]);
    }
    return {};
  };
  jquery.when = function() {
    return {
      done: function() {
        return {fail: function() {}};
      },
    };
  };
  adapterSandbox.$ = jquery;

  vm.runInNewContext(source, adapterSandbox, {filename: sourcePath});
  ready();

  assert.match(alert, /&lt;script&gt;/);
  assert.match(alert, /&lt;host&gt;:3306&quot; onclick=&quot;bad/);
  assert.match(alert, /id\/recovery%3Fx%3D%22bad/);
  assert.doesNotMatch(alert, /<script>|onclick="bad/);
});

test("document adapter renders unavailable state for invalid successful incident payloads", function() {
  [
    [null, {Details: []}, []],
    [[], {}, []],
    [[], {Details: []}, {}],
  ].forEach(function(responses) {
    const rendered = renderAdapterWithResponses(responses[0], responses[1], responses[2]);

    assert.equal(rendered.summary, "Analysis unavailable");
    assert.match(rendered.markup, /Failure analysis is temporarily unavailable/);
  });
});

test("document adapter renders unavailable state when an actionable analysis has no matching cluster", function() {
  const rendered = renderAdapterWithResponses([], {Details: [{
    Analysis: "DeadMaster",
    AnalyzedInstanceKey: {Hostname: "mysql1", Port: 3306},
    ClusterDetails: {ClusterName: "mysql1:3306"},
    CountReplicas: 2,
    IsDowntimed: false,
    StructureAnalysis: [],
  }]}, []);

  assert.equal(rendered.summary, "Analysis unavailable");
  assert.match(rendered.markup, /Failure analysis is temporarily unavailable/);
});
