function failureDetectionRouteSegment(value) {
  return encodeURIComponent(String(value == null ? "" : value));
}

function failureDetectionPagerUrl(baseWebUri, page) {
  return baseWebUri + page;
}

function buildFailureDetectionLink(jquery, label, routePrefix, routeSegment, appUrlFunction) {
  return jquery("<a/>")
    .text(label == null ? "" : label)
    .attr("href", appUrlFunction(routePrefix + failureDetectionRouteSegment(routeSegment)));
}

function buildFailureDetectionMoreInfo(jquery, audit, changelog, appUrlFunction, getInstanceTitleFunction) {
  var container = jquery("<div/>");
  container.append(jquery("<div/>").append(
    jquery("<span/>").text("Detected: "),
    jquery("<span/>").text(audit.RecoveryStartTimestamp)
  ));

  if (audit.AnalysisEntry.Replicas && audit.AnalysisEntry.Replicas.length > 0) {
    var replicas = jquery("<ul/>");
    audit.AnalysisEntry.Replicas.forEach(function(instanceKey) {
      replicas.append(jquery("<li/>").append(
        jquery("<code/>").text(getInstanceTitleFunction(instanceKey.Hostname, instanceKey.Port))
      ));
    });
    container.append(jquery("<div/>").append(
      jquery("<span/>").text(audit.AnalysisEntry.CountReplicas + " replicating hosts :"),
      replicas
    ));
  }

  if (changelog && changelog.length > 0) {
    var changelogList = jquery("<ul/>");
    changelog.slice().reverse().forEach(function(changelogEntry) {
      var changelogEntryTokens = changelogEntry.split(";");
      var changelogEntryTimestamp = changelogEntryTokens[0];
      var changelogEntryAnalysis = changelogEntryTokens.slice(1).join(";");
      if (changelogEntryTimestamp > audit.RecoveryStartTimestamp) {
        return;
      }
      changelogList.append(jquery("<li/>").append(
        jquery("<code/>").append(
          jquery("<span/>").text(changelogEntryTimestamp + " "),
          jquery("<strong/>").text(changelogEntryAnalysis)
        )
      ));
    });
    container.append(jquery("<div/>").append(
      jquery("<span/>").text("Changelog :"),
      changelogList
    ));
  }

  container.append(
    jquery("<div/>").append(buildFailureDetectionLink(
      jquery,
      "Related recovery",
      "/web/audit-recovery/id/",
      audit.RelatedRecoveryId,
      appUrlFunction
    )),
    jquery("<div/>").append(
      jquery("<span/>").text("Processed by "),
      jquery("<code/>").text(audit.ProcessingNodeHostname)
    )
  );
  return container;
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = {
    buildFailureDetectionLink: buildFailureDetectionLink,
    buildFailureDetectionMoreInfo: buildFailureDetectionMoreInfo,
    failureDetectionPagerUrl: failureDetectionPagerUrl,
    failureDetectionRouteSegment: failureDetectionRouteSegment
  };
}

if (typeof document !== "undefined" && typeof $ === "function") {
$(document).ready(function() {
  showLoader();
  var apiUri = "/api/audit-failure-detection/" + currentPage();
  if (clusterAlias() != "") {
    apiUri = "/api/audit-failure-detection/alias/" + failureDetectionRouteSegment(clusterAlias()) + "/" + currentPage();
  } else if (detectionId() > 0) {
    apiUri = "/api/audit-failure-detection/id/" + failureDetectionRouteSegment(detectionId());
  }
  $.get(appUrl(apiUri), function(auditEntries) {
    auditEntries = auditEntries || [];
    $.get(appUrl("/api/replication-analysis-changelog"), function(analysisChangelog) {
      analysisChangelog = analysisChangelog || [];
      displayAudit(auditEntries, analysisChangelog);
    }, "json").fail(displayUnavailable);
  }, "json").fail(displayUnavailable);

  function displayUnavailable() {
    hideLoader();
    $("#audit_empty").hide();
    $("#audit_unavailable").show();
    $("#audit .pager li").addClass("disabled");
  }

  function displayAudit(auditEntries, analysisChangelog) {
    var baseWebUri = appUrl("/web/audit-failure-detection/");
    if (clusterAlias()) {
      baseWebUri += "alias/" + failureDetectionRouteSegment(clusterAlias()) + "/";
    }
    var changelogMap = {}
    analysisChangelog.forEach(function(changelogEntry) {
      changelogMap[getInstanceId(changelogEntry.AnalyzedInstanceKey.Hostname, changelogEntry.AnalyzedInstanceKey.Port)] = changelogEntry.Changelog;
    });

    hideLoader();
	if (auditEntries.length > 0) {
	  $("#audit_empty").hide();
	  $("#audit_table_container").show();
	}
    auditEntries.forEach(function(audit) {
      var analyzedInstanceDisplay = audit.AnalysisEntry.AnalyzedInstanceKey.Hostname + ":" + audit.AnalysisEntry.AnalyzedInstanceKey.Port;
      var row = $('<tr/>');
      var analysisElement = $('<a class="more-detection-info"/>').attr("data-detection-id", audit.Id).text(audit.AnalysisEntry.Analysis);

      $('<td/>').prepend(analysisElement).appendTo(row);
      buildFailureDetectionLink($, analyzedInstanceDisplay, "/web/search/", analyzedInstanceDisplay, appUrl)
        .wrap($("<td/>")).parent().appendTo(row);
      $('<td/>', {
        text: audit.AnalysisEntry.CountReplicas
      }).appendTo(row);
      buildFailureDetectionLink(
        $,
        audit.AnalysisEntry.ClusterDetails.ClusterName,
        "/web/cluster/",
        audit.AnalysisEntry.ClusterDetails.ClusterName,
        appUrl
      ).wrap($("<td/>")).parent().appendTo(row);
      buildFailureDetectionLink(
        $,
        audit.AnalysisEntry.ClusterDetails.ClusterAlias,
        "/web/cluster/alias/",
        audit.AnalysisEntry.ClusterDetails.ClusterAlias,
        appUrl
      ).wrap($("<td/>")).parent().appendTo(row);
      $('<td/>', {
        text: audit.RecoveryStartTimestamp
      }).appendTo(row);

      var changelog = changelogMap[getInstanceId(audit.AnalysisEntry.AnalyzedInstanceKey.Hostname, audit.AnalysisEntry.AnalyzedInstanceKey.Port)];
      var moreInfo = buildFailureDetectionMoreInfo($, audit, changelog, appUrl, getInstanceTitle);
      row.appendTo('#audit tbody');

      var row = $('<tr/>');
      row.attr("data-detection-id-more-info", audit.Id);
      row.addClass("more-info");
      $('<td colspan="6"/>').append(moreInfo).appendTo(row);
      row.hide().appendTo('#audit tbody');
    });
    if (auditEntries.length == 1) {
      $("[data-detection-id-more-info]").show();
    }
    if (currentPage() <= 0) {
      $("#audit .pager .previous").addClass("disabled");
    }
    if (auditEntries.length == 0) {
      $("#audit .pager .next").addClass("disabled");
    }
    $("#audit .pager .previous").not(".disabled").find("a").click(function() {
      window.location.href = failureDetectionPagerUrl(baseWebUri, currentPage() - 1);
    });
    $("#audit .pager .next").not(".disabled").find("a").click(function() {
      window.location.href = failureDetectionPagerUrl(baseWebUri, currentPage() + 1);
    });
    $("#audit .pager .disabled a").click(function() {
      return false;
    });
    $("body").on("click", ".more-detection-info", function(event) {
      var selectedDetectionId = $(event.currentTarget).attr("data-detection-id");
      $("[data-detection-id-more-info]").filter(function() {
        return String($(this).attr("data-detection-id-more-info")) === String(selectedDetectionId);
      }).slideToggle();
    });
  }
});
}
