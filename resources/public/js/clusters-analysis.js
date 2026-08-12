function clusterAnalysisTopologyPath(cluster, compact) {
  var path = '/web/cluster/' + cluster.ClusterName;
  if (cluster.ClusterAlias && cluster.ClusterAlias != cluster.ClusterName) {
    path = '/web/cluster/alias/' + encodeURIComponent(cluster.ClusterAlias);
  }
  return path + (compact ? '?compact=true' : '');
}

function clustersAnalysisBlockedKey(hostname, port, analysis) {
  return hostname + ":" + port + ":" + analysis;
}

function buildClustersAnalysisModel(clusters, replicationAnalysis, blockedRecoveries, interestingAnalysisMap) {
  var unmatchedEntryCount = 0;
  var blocked = {};
  (blockedRecoveries || []).forEach(function(recovery) {
    var key = clustersAnalysisBlockedKey(
      recovery.FailedInstanceKey.Hostname,
      recovery.FailedInstanceKey.Port,
      recovery.Analysis
    );
    blocked[key] = true;
  });

  var byName = {};
  (clusters || []).forEach(function(cluster) {
    byName[cluster.ClusterName] = {
      clusterName: cluster.ClusterName,
      displayName: cluster.ClusterName,
      alias: cluster.ClusterAlias && cluster.ClusterAlias != cluster.ClusterName ? cluster.ClusterAlias : "",
      topologyPath: clusterAnalysisTopologyPath(cluster, true),
      countInstances: cluster.CountInstances,
      allDowntimed: true,
      state: "downtimed",
      entries: [],
    };
  });

  function appendEntry(apiEntry, analysis, structural) {
    var cluster = byName[apiEntry.ClusterDetails.ClusterName];
    if (!cluster) {
      unmatchedEntryCount++;
      return;
    }
    var isBlocked = !!blocked[clustersAnalysisBlockedKey(
      apiEntry.AnalyzedInstanceKey.Hostname,
      apiEntry.AnalyzedInstanceKey.Port,
      analysis
    )];
    var state = structural ? "warning" : (isBlocked ? "blocked" : (apiEntry.IsDowntimed ? "downtimed" : "actionable"));
    var labels = {
      actionable: "Requires attention",
      blocked: "Recovery blocked",
      downtimed: "Downtimed",
      warning: "Structural warning",
    };
    cluster.entries.push({
      analysis: analysis,
      instance: apiEntry.AnalyzedInstanceKey.Hostname + ":" + apiEntry.AnalyzedInstanceKey.Port,
      state: state,
      statusLabel: labels[state],
      impactLabel: structural ? "Participating replicas" : "Affected replicas",
      replicaCount: apiEntry.CountReplicas,
      downtimeEndTimestamp: apiEntry.IsDowntimed ? (apiEntry.DowntimeEndTimestamp || "") : "",
    });
    if (!apiEntry.IsDowntimed) {
      cluster.allDowntimed = false;
    }
  }

  ((replicationAnalysis && replicationAnalysis.Details) || []).forEach(function(entry) {
    if (Object.prototype.hasOwnProperty.call(interestingAnalysisMap, entry.Analysis)) {
      appendEntry(entry, entry.Analysis, false);
    }
    (entry.StructureAnalysis || []).forEach(function(analysis) {
      appendEntry(entry, analysis, true);
    });
  });

  var precedence = {blocked: 4, actionable: 3, warning: 2, downtimed: 1};
  var affected = Object.keys(byName).map(function(name) {
    var cluster = byName[name];
    cluster.entries.sort(function(a, b) {
      var stateOrder = precedence[b.state] - precedence[a.state];
      if (stateOrder) {
        return stateOrder;
      }
      if (a.instance != b.instance) {
        return a.instance < b.instance ? -1 : 1;
      }
      if (a.analysis != b.analysis) {
        return a.analysis < b.analysis ? -1 : 1;
      }
      return 0;
    });
    cluster.entries.forEach(function(entry) {
      if (precedence[entry.state] > precedence[cluster.state]) {
        cluster.state = entry.state;
      }
    });
    return cluster;
  }).filter(function(cluster) {
    return cluster.entries.length > 0;
  });

  affected.forEach(function(cluster) {
    if (typeof removeTextFromHostnameDisplay != "undefined" && removeTextFromHostnameDisplay()) {
      cluster.displayName = cluster.displayName.replace(removeTextFromHostnameDisplay(), '');
    }
  });

  affected.sort(function(a, b) {
    if (a.allDowntimed != b.allDowntimed) {
      return a.allDowntimed ? 1 : -1;
    }
    return (b.countInstances - a.countInstances) || a.clusterName.localeCompare(b.clusterName);
  });

  return {
    clusters: affected,
    incidentCount: affected.reduce(function(total, cluster) { return total + cluster.entries.length; }, 0),
    unmatchedEntryCount: unmatchedEntryCount,
  };
}

function escapeClustersAnalysisHTML(value) {
  return String(value == null ? "" : value).replace(/[&<>"']/g, function(character) {
    return {
      "&": "&amp;",
      "<": "&lt;",
      ">": "&gt;",
      "\"": "&quot;",
      "'": "&#39;",
    }[character];
  });
}

function renderClustersAnalysisMarkup(model) {
  return (model.clusters || []).map(function(cluster) {
    var entries = cluster.entries.map(function(entry) {
      var downtime = entry.downtimeEndTimestamp ? '<p class="analysis-entry-downtime">Downtime ends ' + escapeClustersAnalysisHTML(entry.downtimeEndTimestamp) + '</p>' : '';
      return '<li class="analysis-entry" data-analysis-state="' + escapeClustersAnalysisHTML(entry.state) + '">' +
        '<p class="analysis-entry-analysis">' + escapeClustersAnalysisHTML(entry.analysis) + '</p>' +
        '<p class="analysis-entry-status">' + escapeClustersAnalysisHTML(entry.statusLabel) + '</p>' +
        '<p class="analysis-entry-instance">' + escapeClustersAnalysisHTML(entry.instance) + '</p>' +
        '<p class="analysis-entry-impact">' + escapeClustersAnalysisHTML(entry.impactLabel) + ': ' + escapeClustersAnalysisHTML(entry.replicaCount) + '</p>' +
        downtime +
      '</li>';
    }).join('');
    var alias = cluster.alias ? '<p class="analysis-cluster-alias">' + escapeClustersAnalysisHTML(cluster.alias) + '</p>' : '';

    return '<article class="analysis-cluster" data-analysis-state="' + escapeClustersAnalysisHTML(cluster.state) + '" data-cluster-name="' + escapeClustersAnalysisHTML(cluster.clusterName) + '">' +
      '<header class="analysis-cluster-identity">' +
        '<h2>' + escapeClustersAnalysisHTML(cluster.displayName) + '</h2>' +
        alias +
      '</header>' +
      '<ul class="analysis-entry-list">' + entries + '</ul>' +
      '<aside class="analysis-cluster-impact">' +
        '<p>Instances: ' + escapeClustersAnalysisHTML(cluster.countInstances) + '</p>' +
        '<a href="' + escapeClustersAnalysisHTML(cluster.topologyPath) + '">Open topology</a>' +
      '</aside>' +
    '</article>';
  }).join('');
}

function renderClustersAnalysisEmptyState() {
  return '<section class="clusters-analysis-state clusters-analysis-empty-state" role="status">' +
    '<h2>No active failover incidents</h2>' +
    '<p>All monitored clusters are currently clear.</p>' +
  '</section>';
}

function renderClustersAnalysisUnavailableState() {
  return '<section class="clusters-analysis-state clusters-analysis-unavailable-state" role="alert">' +
    '<h2>Failure analysis is temporarily unavailable</h2>' +
    '<p>Reload the page to try again.</p>' +
    '<a href="">Reload page</a>' +
  '</section>';
}

$(document).ready(function() {
  showLoader();

  var clustersRequest = $.get(appUrl("/api/clusters-info"), null, null, "json");
  var replicationAnalysisRequest = $.get(appUrl("/api/replication-analysis"), null, null, "json");
  var blockedRecoveriesRequest = $.get(appUrl("/api/blocked-recoveries"), null, null, "json");

  function requestData(result) {
    return Array.isArray(result) && typeof result[1] == "string" ? result[0] : result;
  }

  function hasValidClustersAnalysisResponses(clusters, replicationAnalysis, blockedRecoveries) {
    return Array.isArray(clusters) &&
      replicationAnalysis && Array.isArray(replicationAnalysis.Details) &&
      Array.isArray(blockedRecoveries);
  }

  function renderClustersAnalysisState(markup, summary) {
    hideLoader();
    $("#clusters_analysis_loading").hide();
    $("#clusters_analysis_summary").text(summary);
    $("#clusters_analysis_list").html(markup);
  }

  $.when(clustersRequest, replicationAnalysisRequest, blockedRecoveriesRequest)
    .done(function(clustersResult, replicationAnalysisResult, blockedRecoveriesResult) {
      var clusters = requestData(clustersResult);
      var replicationAnalysis = requestData(replicationAnalysisResult);
      var blockedRecoveries = requestData(blockedRecoveriesResult);
      if (!hasValidClustersAnalysisResponses(clusters, replicationAnalysis, blockedRecoveries)) {
        renderClustersAnalysisState(renderClustersAnalysisUnavailableState(), "Analysis unavailable");
        return;
      }
      var model = buildClustersAnalysisModel(
        clusters,
        replicationAnalysis,
        blockedRecoveries,
        interestingAnalysis
      );
      if (model.unmatchedEntryCount > 0) {
        renderClustersAnalysisState(renderClustersAnalysisUnavailableState(), "Analysis unavailable");
        return;
      }
      model.clusters.forEach(function(cluster) {
        cluster.topologyPath = appUrl(cluster.topologyPath);
      });
      var incidents = model.incidentCount;
      var clusterCount = model.clusters.length;
      var summary = incidents + ' active incident' + (incidents == 1 ? '' : 's') + ' across ' + clusterCount + ' cluster' + (clusterCount == 1 ? '' : 's');
      renderClustersAnalysisState(
        incidents ? renderClustersAnalysisMarkup(model) : renderClustersAnalysisEmptyState(),
        summary
      );
    })
    .fail(function() {
      renderClustersAnalysisState(renderClustersAnalysisUnavailableState(), "Analysis unavailable");
    });

  $.get(appUrl("/api/blocked-recoveries"), function(blockedRecoveries) {
    blockedRecoveries = blockedRecoveries || [];
    // Result is an array: either empty (no active recovery) or with multiple entries
    blockedRecoveries.forEach(function(blockedRecovery) {
      var instanceTitle = getInstanceTitle(blockedRecovery.FailedInstanceKey.Hostname, blockedRecovery.FailedInstanceKey.Port);
      var auditPath = '/web/audit-recovery/id/' + encodeURIComponent(blockedRecovery.BlockingRecoveryId);
      addAlert('A <strong>' + escapeClustersAnalysisHTML(blockedRecovery.Analysis) + '</strong> on ' + escapeClustersAnalysisHTML(instanceTitle) + ' is blocked due to a <a href="' + escapeClustersAnalysisHTML(appUrl(auditPath)) + '">previous recovery</a>');
    });
  });

  if (isAuthorizedForAction()) {
    // Read-only users don't get auto-refresh. Sorry!
    activateRefreshTimer();
  }
});
