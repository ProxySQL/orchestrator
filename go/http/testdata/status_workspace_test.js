const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const sourcePath = path.resolve(__dirname, "../../../resources/public/js/status.js");
const source = fs.readFileSync(sourcePath, "utf8");

function loadStatusPage(options = {}) {
  const state = new Map();
  const apiCalls = [];
  const healthRequests = [];
  const health = {
    Message: "Application node is healthy",
    Details: {
      AvailableNodes: [],
      ActiveNode: {},
      Hostname: "orchestrator",
      RaftLeader: "",
      RaftHealthyMembers: [],
    },
  };

  function selection(selector) {
    if (!state.has(selector)) {
      state.set(selector, {
        appended: [],
        prepended: [],
        click: null,
        hidden: selector === "#status_content" || selector === "#status_unavailable",
      });
    }
    const value = state.get(selector);
    return {
      append(html) { value.appended.push(html); return this; },
      prepend(html) { value.prepended.push(html); return this; },
      click(handler) { value.click = handler; return this; },
      hide() { value.hidden = true; return this; },
      show() { value.hidden = false; return this; },
      text(message) { value.text = message; return this; },
      ready(handler) { handler(); return this; },
    };
  }

  const document = {};
  function $(selector) {
    return selection(selector === document ? "document" : selector);
  }
  $.get = function(uri, callback) {
    healthRequests.push(uri);
    if (!options.failureResponse) callback(health);
    return {
      fail(handler) {
        if (options.failureResponse) handler({responseJSON: options.failureResponse});
        return this;
      },
    };
  };

  const sandbox = {
    document,
    $,
    appUrl: value => value,
    getUserId: () => "",
    isAuthorizedForAction: () => false,
    apiCommand: uri => apiCalls.push(uri),
  };
  vm.runInNewContext(source, sandbox, {filename: sourcePath});
  return {sandbox, state, apiCalls, healthRequests};
}

test("status page loads the registered health endpoint into its summary", function() {
  const page = loadStatusPage();

  assert.deepEqual(page.healthRequests, ["/api/health"]);
  assert.equal(page.state.get("#status_summary").text, "Application node is healthy");
});

test("status actions render in the Bootstrap card footer and invoke their API command", function() {
  const page = loadStatusPage();

  page.sandbox.addStatusActionButton("Reload configuration", "reload-configuration");
  assert.match(page.state.get("#orchestratorStatus .card-footer").appended.join(""), /Reload configuration/);
  page.state.get("#orchestratorStatus .card-footer button:last").click();
  assert.deepEqual(page.apiCalls, ["/api/reload-configuration"]);
});

test("status page renders an unhealthy response from the reachable health endpoint", function() {
  const page = loadStatusPage({
    failureResponse: {
      Code: "ERROR",
      Message: "Application node is unhealthy: backend check failed",
      Details: {
        AvailableNodes: [],
        ActiveNode: {},
        Hostname: "orchestrator",
        RaftLeader: "",
        RaftHealthyMembers: [],
      },
    },
  });

  assert.equal(page.state.get("#status_summary").text, "Application node is unhealthy: backend check failed");
  assert.equal(page.state.get("#status_content").hidden, false);
  assert.equal(page.state.get("#status_unavailable").hidden, true);
});
