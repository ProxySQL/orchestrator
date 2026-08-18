function resolveClusterHealthSummary(analysisState, problemBadgeClasses) {
  var statePriority = {
    "healthy": 0,
    "maintenance": 1,
    "stale": 2,
    "warning": 3,
    "errant": 3,
    "danger": 4,
    "fatal": 5,
  };
  var badgeState = {
    "label-info": "maintenance",
    "label-stale": "stale",
    "label-warning": "warning",
    "label-errant": "errant",
    "label-danger": "danger",
    "label-fatal": "fatal",
  };
  var state = statePriority[analysisState] === undefined ? "healthy" : analysisState;

  (problemBadgeClasses || []).forEach(function(badgeClass) {
    var candidateState = badgeState[badgeClass];
    if (candidateState && statePriority[candidateState] > statePriority[state]) {
      state = candidateState;
    }
  });

  var label = {
    "healthy": "No problems",
    "maintenance": "Maintenance",
    "stale": "Stale",
    "warning": "Problems",
    "errant": "Problems",
    "danger": "Problems",
    "fatal": "Problems",
  }[state];

  return {state: state, label: label};
}

function clusterTopologyPath(cluster) {
  if (cluster.ClusterAlias && cluster.ClusterAlias != cluster.ClusterName) {
    return '/web/cluster/alias/' + encodeURIComponent(cluster.ClusterAlias) + '?compact=true';
  }
  return '/web/cluster/' + cluster.ClusterName + '?compact=true';
}

$(document).ready(function() {
  showLoader();

  $.get(appUrl("/api/clusters-info"), function(clusters) {
    $.get(appUrl("/api/replication-analysis"), function(replicationAnalysis) {
      $.get(appUrl("/api/problems"), function(problemInstances) {
        if (problemInstances == null) {
          problemInstances = [];
        }
        normalizeInstances(problemInstances, []);
        displayClusters(clusters, replicationAnalysis, problemInstances);
      }, "json");
    }, "json");
  }, "json");

  function sortByCountInstances(cluster1, cluster2) {
    var diff = cluster2.CountInstances - cluster1.CountInstances;
    if (diff != 0) {
      return diff;
    }
    return cluster1.ClusterName.localeCompare(cluster2.ClusterName);
  }

  function sortByClusterName(cluster1, cluster2) {
    return cluster1.ClusterName.localeCompare(cluster2.ClusterName);
  }

  function sortByClusterAlias(cluster1, cluster2) {
    return cluster1.ClusterAlias.localeCompare(cluster2.ClusterAlias);
  }

  function displayClusters(clusters, replicationAnalysis, problemInstances) {
    hideLoader();

    clusters = clusters || [];

    var dashboardSort = $.cookie("dashboard-sort") || "count"

    refreshDashboardSortButton();
    $("#li-dashboard-sort").appendTo("ul.navbar-nav").show();
    $("#dashboard-sort a").click(function() {
      $.cookie("dashboard-sort", $(this).attr("dashboard-sort"), {
        path: '/',
        expires: 3650
      });
      location.reload();
    });

    var clustersProblems = {};
    clusters.forEach(function(cluster) {
      clustersProblems[cluster.ClusterName] = {};
    });

    var clustersAnalysisProblems = {};
    replicationAnalysis.Details.forEach(function(analysisEntry) {
      if (!clustersAnalysisProblems[analysisEntry.ClusterDetails.ClusterName]) {
        clustersAnalysisProblems[analysisEntry.ClusterDetails.ClusterName] = [];
      }
      if (analysisEntry.Analysis in interestingAnalysis) {
        clustersAnalysisProblems[analysisEntry.ClusterDetails.ClusterName].push(analysisEntry);
      }
      analysisEntry.StructureAnalysis = analysisEntry.StructureAnalysis || [];
      analysisEntry.StructureAnalysis.forEach(function(structureAnalysis) {
        // We don't need a deep clone. Shallow copy is enough.
        const analysisEntryClone = {...analysisEntry};
        analysisEntryClone.Analysis = structureAnalysis;
        analysisEntryClone.IsStructureAnalysis = true;
        clustersAnalysisProblems[analysisEntryClone.ClusterDetails.ClusterName].push(analysisEntryClone);
      });
    });

    function refreshDashboardSortButton() {
      if (dashboardSort == "name") {
        clusters.sort(sortByClusterName);
      } else if (dashboardSort == "alias") {
        clusters.sort(sortByClusterAlias);
      } else {
        clusters.sort(sortByCountInstances);
      }

      $("#dashboard-sort-button").html("Sort by " + dashboardSort + ' <span class="caret"></span>')
    }

    function addInstancesBadge(clusterName, count, badgeClass, title) {
      $("#clusters [data-cluster-name='" + clusterName + "'].popover").find(".popover-content .pull-right").append('<span class="badge ' + badgeClass + '" title="' + title + '">' + count + '</span> ');
    }

    function incrementClusterProblems(clusterName, problemType) {
      if (!problemType) {
        return
      }
      if (clustersProblems[clusterName][problemType] > 0) {
        clustersProblems[clusterName][problemType] = clustersProblems[clusterName][problemType] + 1;
      } else {
        clustersProblems[clusterName][problemType] = 1;
      }
    }
    problemInstances.forEach(function(instance) {
      incrementClusterProblems(instance.ClusterName, instance.problem)
    });

    clusters.forEach(function(cluster) {
      $("#clusters").append('<div xmlns="http://www.w3.org/1999/xhtml" class="popover instance right" data-cluster-name="' + cluster.ClusterName + '"><div class="arrow"></div><h3 class="popover-title"><div class="pull-left"><a href="' + appUrl('/web/cluster/' + cluster.ClusterName) + '"><span>' + cluster.ClusterName + '</span></a></div><div class="pull-right"></div>&nbsp;<br/>&nbsp;</h3><div class="popover-content"></div></div>');
      var popoverElement = $("#clusters [data-cluster-name='" + cluster.ClusterName + "'].popover");
      var analysisState = "healthy";

      if (typeof removeTextFromHostnameDisplay != "undefined" && removeTextFromHostnameDisplay()) {
        var title = cluster.ClusterName.replace(removeTextFromHostnameDisplay(), '');
        popoverElement.find("h3 .pull-left a span").html(title);
      }
      var compactClusterUri = appUrl(clusterTopologyPath(cluster));
      if (cluster.ClusterAlias && cluster.ClusterAlias != cluster.ClusterName) {
        popoverElement.find("h3 .pull-left a span").addClass("small");
        popoverElement.find("h3 .pull-left").prepend('<a href="' + appUrl('/web/cluster/alias/' + encodeURIComponent(cluster.ClusterAlias)) + '"><strong>' + cluster.ClusterAlias + '</strong></a><br/>');
      }
      if (clustersAnalysisProblems[cluster.ClusterName]) {
        var mutedMsg = ""
        var mutedCnt = 0
        var warningMsg = ""
        var warningCnt = 0
        var dangerMsg = ""
        var dangerCnt = 0

        clustersAnalysisProblems[cluster.ClusterName].forEach(function(analysisEntry) {          
          var msg = analysisEntry.Analysis + ': ' + getInstanceTitle(analysisEntry.AnalyzedInstanceKey.Hostname, analysisEntry.AnalyzedInstanceKey.Port)

          var hasDowntime = analysisEntry.IsDowntimed || analysisEntry.IsReplicasDowntimed
          if (hasDowntime) {
            mutedMsg = mutedMsg.concat(msg, '\n');
            mutedCnt++;
          } else if (analysisEntry.IsStructureAnalysis) {
            warningMsg = warningMsg.concat(msg, '\n');
            warningCnt++;
          } else {
            dangerMsg = dangerMsg.concat(msg, '\n');
            dangerCnt++;
          }
        });
        if (mutedCnt > 0) {
          analysisState = "maintenance";
          if (mutedCnt > 1) {
            popoverElement.find("h3 .pull-left").prepend('<span class="overlay-counter">' + mutedCnt +' </span>');
          }
          popoverElement.find("h3 .pull-left").prepend('<i class="bi bi-exclamation-triangle-fill text-muted"' + ' title="' + mutedMsg + '" aria-hidden="true"></i>');
        }
        if (warningCnt > 0) {
          analysisState = "warning";
          if (warningCnt > 1) {
            popoverElement.find("h3 .pull-left").prepend('<span class="overlay-counter">' + warningCnt +' </span>');
          }
          popoverElement.find("h3 .pull-left").prepend('<i class="bi bi-exclamation-triangle-fill text-warning"' + ' title="' + warningMsg + '" aria-hidden="true"></i>');
        }
        if (dangerCnt > 0) {
          analysisState = "danger";
          if (dangerCnt > 1) {
            popoverElement.find("h3 .pull-left").prepend('<span class="overlay-counter">' + dangerCnt +' </span>');
          }
          popoverElement.find("h3 .pull-left").prepend('<i class="bi bi-exclamation-triangle-fill text-danger"' + ' title="' + dangerMsg + '" aria-hidden="true"></i>');
        }
      }
      popoverElement.find("h3 .pull-right").append('<a href="' + compactClusterUri + '" aria-label="Open compact topology" title="Compact display"><i class="bi bi-diagram-3" aria-hidden="true"></i></a>');
      if (cluster.HasAutomatedIntermediateMasterRecovery === true) {
        popoverElement.find("h3 .pull-right").prepend('<i class="bi bi-check-circle-fill text-info" title="Automated intermediate master recovery for this cluster ENABLED" aria-hidden="true"></i>');
      }
      if (cluster.HasAutomatedMasterRecovery === true) {
        popoverElement.find("h3 .pull-right").prepend('<i class="bi bi-check-circle-fill text-info" title="Automated master recovery for this cluster ENABLED" aria-hidden="true"></i>');
      }

      var clusterProblemTypes = [];
      var clusterProblemBadgeClasses = [];
      for (var problemType in clustersProblems[cluster.ClusterName]) {
        clusterProblemTypes.push(problemType);
        clusterProblemBadgeClasses.push(errorMapping[problemType]["badge"]);
      }
      var healthSummary = resolveClusterHealthSummary(analysisState, clusterProblemBadgeClasses);
      popoverElement.attr("data-health-state", healthSummary.state);

      var contentHtml = '' +
        '<div><span class="cluster-members-label">Members</span><div class="pull-right"></div>' +
        '<span class="cluster-health-label">' + healthSummary.label + '</span></div>';
      popoverElement.find(".popover-content").html(contentHtml);
      addInstancesBadge(cluster.ClusterName, cluster.CountInstances, "label-primary", "Total instances in cluster");
      clusterProblemTypes.forEach(function(problemType) {
        addInstancesBadge(cluster.ClusterName, clustersProblems[cluster.ClusterName][problemType], errorMapping[problemType]["badge"], errorMapping[problemType]["description"]);
      });
    });

    $("div.popover").popover();
    $("div.popover").show();

    if (clusters.length == 0) {
      addAlert("No clusters found");
    }
  }

  if (isAuthorizedForAction()) {
    // Read-only users don't get auto-refresh. Sorry!
    activateRefreshTimer();
  }
});
