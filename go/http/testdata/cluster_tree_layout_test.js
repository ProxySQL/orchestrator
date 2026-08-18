const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

test('D3 v7 layout recalculates coordinates without replacing transition origins', () => {
  const context = {globalThis: null, window: null};
  context.globalThis = context;
  context.window = context;
  vm.createContext(context);
  vm.runInContext('Array.from = function(arrayLike) { return Array.prototype.slice.call(arrayLike, 0); };', context);
  vm.runInContext(fs.readFileSync(path.join(__dirname, '../../../resources/public/js/d3.v7.min.js'), 'utf8'), context);
  vm.runInContext(fs.readFileSync(path.join(__dirname, '../../../resources/public/js/cluster-tree-layout.js'), 'utf8'), context);

  const replica1 = {id: 'replica1', virtualDepth: 1, children: [], x0: 123, y0: 320};
  const replica2 = {id: 'replica2', virtualDepth: 1, children: []};
  const root = {id: 'primary', virtualDepth: 0, children: [replica1, replica2]};
  const tree = context.d3.tree().size([600, 640]);
  const first = context.OrchestratorTreeLayout.layout(context.d3, root, tree, 320);
  assert.deepEqual(
    {nodeCount: first.nodes.length, linkCount: first.links.length},
    {nodeCount: 3, linkCount: 2}
  );
  const firstByID = Object.fromEntries(first.nodes.map(node => [node.id, node]));
  const firstReplicaX = firstByID.replica1.x;

  root.children = [replica1];
  const second = context.OrchestratorTreeLayout.layout(context.d3, root, tree, 320);
  const secondByID = Object.fromEntries(second.nodes.map(node => [node.id, node]));

  assert.equal(firstByID.replica1.y, 320);
  assert.notEqual(secondByID.replica1.x, firstReplicaX);
  assert.equal(secondByID.replica1.x0, 123);
  assert.equal(secondByID.replica1.y0, 320);
});
