const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const sourcePath = path.resolve(__dirname, "../../../resources/public/js/instance-problems.js");
const source = fs.readFileSync(sourcePath, "utf8");
const sandbox = {
  document: {},
  $: function() {
    return {ready: function() {}};
  },
};

vm.runInNewContext(source, sandbox, {filename: sourcePath});

test("cluster workspace keeps problem details closed until requested", function() {
  assert.equal(sandbox.shouldAutoShowProblemDropdown(2, true, false, true), false);
});

test("other pages retain configured problem auto-show behavior", function() {
  assert.equal(sandbox.shouldAutoShowProblemDropdown(2, true, false, false), true);
  assert.equal(sandbox.shouldAutoShowProblemDropdown(0, true, false, false), false);
  assert.equal(sandbox.shouldAutoShowProblemDropdown(2, false, false, false), false);
  assert.equal(sandbox.shouldAutoShowProblemDropdown(2, true, true, false), false);
});
