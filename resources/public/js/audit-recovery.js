function recoveryRouteSegment(value) {
  return encodeURIComponent(String(value == null ? "" : value));
}

function resolveRecoveryViewState(auditEntries, recoveryIdValue, recoveryUidValue, unavailable) {
  if (unavailable) {
    return "unavailable";
  }
  if (!Array.isArray(auditEntries) || auditEntries.length === 0) {
    return "empty";
  }
  if (Number(recoveryIdValue) > 0 || String(recoveryUidValue || "") !== "") {
    return "detail";
  }
  return "list";
}

function buildRecoveryAuditLink(jquery, label, routePrefix, routeSegment, appUrlFunction) {
  return jquery("<a/>")
    .text(label == null ? "" : label)
    .attr("href", appUrlFunction(routePrefix + recoveryRouteSegment(routeSegment)));
}

function buildRecoveryAcknowledgement(jquery, audit) {
  var container = jquery("<div/>");
  if (audit.Acknowledged) {
    container.append(
      jquery("<span/>")
        .addClass("text-success")
        .text("Acknowledged by " + audit.AcknowledgedBy + ", " + audit.AcknowledgedAt),
      jquery("<ul/>").append(jquery("<li/>").text(audit.AcknowledgedComment))
    );
  } else {
    container.append(
      jquery("<button/>")
        .addClass("btn btn-primary ack-recovery")
        .attr("type", "button")
        .attr("data-recovery-id", audit.Id)
        .text("Acknowledge"),
      jquery("<span/>").text(" This recovery is unacknowledged.")
    );
  }
  return container;
}

function buildRecoveryAuditInfo(jquery, audit, appUrlFunction, getInstanceTitleFunction) {
  var container = jquery("<div/>");
  var appendInstanceList = function(label, instanceKeys) {
    if (!instanceKeys || instanceKeys.length === 0) {
      return;
    }
    var list = jquery("<ul/>");
    instanceKeys.forEach(function(instanceKey) {
      list.append(jquery("<li/>").append(
        jquery("<code/>").text(getInstanceTitleFunction(instanceKey.Hostname, instanceKey.Port))
      ));
    });
    container.append(jquery("<div/>").append(jquery("<span/>").text(label), list));
  };

  appendInstanceList("Lost replicas:", audit.LostReplicas);
  appendInstanceList("Participating instances:", audit.ParticipatingInstanceKeys);
  appendInstanceList(audit.AnalysisEntry.CountReplicas + " replicating hosts :", audit.AnalysisEntry.Replicas);

  if (audit.AllErrors && audit.AllErrors.length > 0 && audit.AllErrors[0]) {
    var errors = jquery("<ul/>");
    audit.AllErrors.forEach(function(error) {
      errors.append(jquery("<li/>").text(error));
    });
    container.append(jquery("<div/>").append(jquery("<span/>").text("All errors:"), errors));
  }

  container.append(
    jquery("<div/>").append(buildRecoveryAuditLink(
      jquery,
      "Related detection",
      "/web/audit-failure-detection/id/",
      audit.LastDetectionId,
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
    buildRecoveryAcknowledgement: buildRecoveryAcknowledgement,
    buildRecoveryAuditInfo: buildRecoveryAuditInfo,
    buildRecoveryAuditLink: buildRecoveryAuditLink,
    recoveryRouteSegment: recoveryRouteSegment,
    resolveRecoveryViewState: resolveRecoveryViewState
  };
}

if (typeof document !== "undefined" && typeof $ === "function") {
$(document).ready(function() {
  $("#audit_recovery_steps").hide();
  showLoader();
  var apiUri = "/api/audit-recovery/" + currentPage();
  if (clusterName()) {
    apiUri = "/api/audit-recovery/cluster/" + recoveryRouteSegment(clusterName()) + "/" + currentPage();
  } else if (clusterAlias()) {
    apiUri = "/api/audit-recovery/alias/" + recoveryRouteSegment(clusterAlias()) + "/" + currentPage();
  } else if (recoveryId() > 0) {
    apiUri = "/api/audit-recovery/id/" + recoveryRouteSegment(recoveryId());
  } else if (recoveryUid()) {
    apiUri = "/api/audit-recovery/uid/" + recoveryRouteSegment(recoveryUid());
  }
  $.get(appUrl(apiUri), function(auditEntries) {
    auditEntries = auditEntries || [];
    displayAudit(auditEntries);
  }, "json").fail(function() {
    hideLoader();
    $("#audit_empty").hide();
    $("#audit_unavailable").show();
    $("#audit .pager li").addClass("disabled");
  });

  function displaySingleAudit(audit) {
    $("#audit .pager").hide();
    $("#audit_recovery_table").hide();
	$("#audit_recovery_details_container").show();

    var clusterAlias = audit.AnalysisEntry.ClusterDetails.ClusterAlias;
    var clusterName = audit.AnalysisEntry.ClusterDetails.ClusterName;
    var failedInstanceTitle = getInstanceTitle(audit.AnalysisEntry.AnalyzedInstanceKey.Hostname, audit.AnalysisEntry.AnalyzedInstanceKey.Port);
    $("#audit_recovery_details thead h3").text(audit.AnalysisEntry.Analysis + ' on ' + clusterAlias + '/' + failedInstanceTitle)

    var appendRow = function(td1, td2) {
      var row = $('<tr/>');
      $('<td/>', {
        text: td1
      }).appendTo(row);
      var valueCell = $('<td/>');
      if (td2 && typeof td2.appendTo === "function") {
        td2.appendTo(valueCell);
      } else {
        valueCell.text(td2 == null ? "" : td2);
      }
      valueCell.appendTo(row);

      row.appendTo($("#audit_recovery_details tbody"));
    }
    appendRow("Failed instance", failedInstanceTitle)
    var successorTitle = getInstanceTitle(audit.SuccessorKey.Hostname, audit.SuccessorKey.Port);
    var successor = $('<span/>');
    if (audit.IsSuccessful === false) {
      successor.addClass("text-danger").append(
        $('<i/>').addClass("bi bi-x-circle-fill").attr("aria-hidden", "true"),
        $('<span/>').text(" FAIL " + successorTitle)
      );
    } else {
      successor.addClass("text-success").append(
        $('<i/>').addClass("bi bi-check-circle-fill").attr("aria-hidden", "true"),
        $('<span/>').text(" " + successorTitle)
      );
    }
    appendRow("Successor", successor)
    if (clusterAlias != clusterName) {
      appendRow("Cluster alias", buildRecoveryAuditLink($, clusterAlias, "/web/cluster/alias/", clusterAlias, appUrl))
    }
    appendRow("Cluster name", buildRecoveryAuditLink($, clusterName, "/web/cluster/", clusterName, appUrl))
    appendRow("Affected replicas", audit.AnalysisEntry.CountReplicas)
    appendRow("Start time", audit.RecoveryStartTimestamp)
    appendRow("End time", audit.RecoveryEndTimestamp)

    var numRows = $("#audit_recovery_details tbody tr").length;
    $('<td/>').append(buildRecoveryAcknowledgement($, audit))
      .attr("rowspan", 1).addClass("ack").appendTo($("#audit_recovery_details tbody tr:first-child"));
    $('<td/>').append(buildRecoveryAuditInfo($, audit, appUrl, getInstanceTitle))
      .attr("rowspan", numRows-1).appendTo($("#audit_recovery_details tbody tr:nth-child(2)"));

    auditRecoverySteps(recoveryRouteSegment(audit.UID), $('#audit_recovery_steps'))
	$("#audit_recovery_steps_container").show();
	$("#audit_recovery_steps").show();
  }

  function displayAudit(auditEntries) {
    var baseWebUri = appUrl("/web/audit-recovery/");
    if (clusterName()) {
      baseWebUri += "cluster/" + recoveryRouteSegment(clusterName()) + "/";
    } else if (clusterAlias()) {
      baseWebUri += "alias/" + recoveryRouteSegment(clusterAlias()) + "/";
    }
    var viewState = resolveRecoveryViewState(auditEntries, recoveryId(), recoveryUid(), false);

    hideLoader();
	if (auditEntries.length > 0) {
	  $("#audit_empty").hide();
	  if (viewState === "list") {
		$("#audit_recovery_table_container").show();
	  }
	}
    if (viewState === "detail") {
      displaySingleAudit(auditEntries[0]);
    } else if (recoveryId() > 0 || recoveryUid()) {
      $("#audit .pager").hide();
    }
    auditEntries.forEach(function(audit) {
      if (viewState !== "list") {
        return;
      }

      var analyzedInstanceDisplay = getInstanceTitle(audit.AnalysisEntry.AnalyzedInstanceKey.Hostname, audit.AnalysisEntry.AnalyzedInstanceKey.Port);
      var sucessorInstanceDisplay = getInstanceTitle(audit.SuccessorKey.Hostname, audit.SuccessorKey.Port);
      var row = $('<tr/>');
      var ack;
      if (audit.Acknowledged) {
        var ackTitle = "Acknowledged by " + audit.AcknowledgedBy + " at " + audit.AcknowledgedAt + ": " + audit.AcknowledgedComment;
        ack = $('<span class="acknowledge-indicator" role="img"><i class="bi bi-check-circle-fill text-primary" aria-hidden="true"></i></span>');
        ack.attr("title", ackTitle).attr("aria-label", ackTitle);
      } else {
        ack = $('<button type="button" class="acknowledge-indicator ack-recovery" aria-label="Acknowledge recovery"><i class="bi bi-exclamation-triangle-fill text-danger" aria-hidden="true"></i></button>');
        ack.attr("data-recovery-id", audit.Id);
        ack.attr("title", "Unacknowledged. Click to acknowledge");
      }

      $('<td/>').append(buildRecoveryAuditLink(
        $,
        audit.AnalysisEntry.Analysis,
        "/web/audit-recovery/uid/",
        audit.UID,
        appUrl
      )).prepend(ack).appendTo(row);
      buildRecoveryAuditLink($, analyzedInstanceDisplay, "/web/search/", analyzedInstanceDisplay, appUrl)
        .wrap($("<td/>")).parent().appendTo(row);
      $('<td/>', {
        text: audit.AnalysisEntry.CountReplicas
      }).appendTo(row);
      buildRecoveryAuditLink(
        $,
        audit.AnalysisEntry.ClusterDetails.ClusterName,
        "/web/cluster/",
        audit.AnalysisEntry.ClusterDetails.ClusterName,
        appUrl
      ).wrap($("<td/>")).parent().appendTo(row);
      buildRecoveryAuditLink(
        $,
        audit.AnalysisEntry.ClusterDetails.ClusterAlias,
        "/web/cluster/alias/",
        audit.AnalysisEntry.ClusterDetails.ClusterAlias,
        appUrl
      ).wrap($("<td/>")).parent().appendTo(row);
      $('<td/>', {
        text: audit.RecoveryStartTimestamp
      }).appendTo(row);
      $('<td/>', {
        text: audit.RecoveryEndTimestamp
      }).appendTo(row);
      if (audit.RecoveryEndTimestamp && !audit.IsSuccessful && !audit.SuccessorKey.Hostname) {
        $('<td/>', {
          text: "FAIL"
        }).appendTo(row);
      } else if (audit.SuccessorKey.Hostname) {
        buildRecoveryAuditLink($, sucessorInstanceDisplay, "/web/search/", sucessorInstanceDisplay, appUrl)
          .wrap($("<td/>")).parent().appendTo(row);
      } else {
        $('<td/>', {
          text: "pending"
        }).appendTo(row);
      }
      var moreInfo = buildRecoveryAuditInfo($, audit, appUrl, getInstanceTitle);
      row.appendTo('#audit_recovery_table tbody');

      var row = $('<tr/>');
      row.addClass("more-info");
      row.attr("data-recovery-id-more-info", audit.Id);
      $('<td colspan="8"/>').append(moreInfo).appendTo(row);
      if (audit.Acknowledged) {
        row.hide()
      }
      row.appendTo('#audit_recovery_table tbody');
    });
    if (currentPage() <= 0) {
      $("#audit .pager .previous").addClass("disabled");
    }
    if (auditEntries.length == 0) {
      $("#audit .pager .next").addClass("disabled");
    }
    $("#audit .pager .previous").not(".disabled").find("a").click(function() {
      window.location.href = baseWebUri + (currentPage() - 1);
    });
    $("#audit .pager .next").not(".disabled").find("a").click(function() {
      window.location.href = baseWebUri + (currentPage() + 1);
    });
    $("#audit .pager .disabled a").click(function() {
      return false;
    });
    $("body").on("click", ".ack-recovery", function(event) {
      var recoveryId = $(event.currentTarget).attr("data-recovery-id");
      bootbox.prompt({
        title: "Acknowledge recovery",
        placeholder: "comment",
        callback: function(result) {
          if (result !== null) {
            apiCommand("/api/ack-recovery/" + recoveryRouteSegment(recoveryId) + "?comment=" + encodeURIComponent(result));
          }
        }
      });
    });
  }
});
}
