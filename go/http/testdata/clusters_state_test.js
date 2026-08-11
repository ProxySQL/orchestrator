const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const sourcePath = path.resolve(__dirname, "../../../resources/public/js/clusters.js");
const source = fs.readFileSync(sourcePath, "utf8");
const sandbox = {
  document: {},
  $: function() {
    return {ready: function() {}};
  },
};

vm.runInNewContext(source, sandbox, {filename: sourcePath});

test("analysis warnings produce a non-healthy cluster summary", function() {
  const summary = sandbox.resolveClusterHealthSummary("warning", []);

  assert.equal(summary.state, "warning");
  assert.equal(summary.label, "Problems");
});

test("problem badge severity wins according to operational precedence", function() {
  const fatal = sandbox.resolveClusterHealthSummary("maintenance", ["label-info", "label-fatal"]);
  const errant = sandbox.resolveClusterHealthSummary("healthy", ["label-errant"]);
  const stale = sandbox.resolveClusterHealthSummary("maintenance", ["label-stale"]);

  assert.equal(fatal.state, "fatal");
  assert.equal(fatal.label, "Problems");
  assert.equal(errant.state, "errant");
  assert.equal(errant.label, "Problems");
  assert.equal(stale.state, "stale");
  assert.equal(stale.label, "Stale");
});

test("a cluster without analysis or problem signals reports no problems", function() {
  const summary = sandbox.resolveClusterHealthSummary("healthy", ["label-primary"]);

  assert.equal(summary.state, "healthy");
  assert.equal(summary.label, "No problems");
});
