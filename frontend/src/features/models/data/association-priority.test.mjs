import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import {
  MAX_ASSOCIATION_PRIORITY,
  associationPrioritySchema,
  buildPriorityTiers,
  countPriorityTiers,
  insertRowAsNewTier,
  moveRowToTier,
  priorityByRow,
} from './association-priority.ts';

function firstIssue(value) {
  const result = associationPrioritySchema.safeParse(value);
  return result.success ? null : result.error.issues[0].message;
}

/** Builds tiers from `{ id, priority }` rows, mirroring how the dialog does it. */
function tiersOf(rows) {
  return buildPriorityTiers(
    rows,
    (row) => row.id,
    (row) => row.priority
  );
}

/** Renders tiers as `{ id: tier }` so assertions read like the UI (`P0`, `P1`). */
function prioritiesOf(tiers) {
  return Object.fromEntries(priorityByRow(tiers));
}

/** Every priority in a non-empty tier list must be a dense 0..n-1 range. */
function assertDense(tiers) {
  const seen = new Set();
  for (const tier of tiers) {
    for (const id of tier) {
      assert.ok(!seen.has(id), `row ${id} appears in more than one tier`);
      seen.add(id);
    }
  }

  const distinct = [...new Set(priorityByRow(tiers).values())].sort((a, b) => a - b);
  assert.deepEqual(
    distinct,
    distinct.map((_, index) => index),
    'tiers must be numbered densely from 0'
  );
  assert.equal(distinct.length, tiers.length, 'every tier must have its own priority');
}

test('association priority accepts the documented 0-100 range', () => {
  assert.equal(MAX_ASSOCIATION_PRIORITY, 100);
  for (const priority of [0, 1, 10, 50, 99, 100]) {
    assert.equal(firstIssue(priority), null, `priority ${priority} should be valid`);
  }
});

test('association priority reports out-of-range values instead of clamping them', () => {
  assert.equal(firstIssue(101), 'Priority cannot exceed 100');
  assert.equal(firstIssue(-1), 'Priority must be at least 0');
  assert.equal(firstIssue(1.5), 'Priority must be an integer');
  assert.equal(firstIssue(null), 'Priority is required');
});

test('tiers are ordered by priority and keep the rows that share one', () => {
  const tiers = tiersOf([
    { id: 'a', priority: 0 },
    { id: 'b', priority: 1 },
    { id: 'c', priority: 0 },
  ]);

  assert.deepEqual(tiers, [['a', 'c'], ['b']]);
  assert.deepEqual(prioritiesOf(tiers), { a: 0, c: 0, b: 1 });
  assertDense(tiers);
});

test('unnumbered legacy priorities collapse into the same dense tiers', () => {
  const tiers = tiersOf([
    { id: 'a', priority: 3 },
    { id: 'b', priority: 1 },
    { id: 'c', priority: 50 },
    { id: 'd', priority: 1 },
  ]);

  // Order comes from the priority, and the numbering is rewritten to 0..n-1.
  assert.deepEqual(tiers, [['b', 'd'], ['a'], ['c']]);
  assertDense(tiers);
});

test('missing priorities are grouped instead of dropping the row', () => {
  const tiers = tiersOf([
    { id: 'a', priority: null },
    { id: 'b', priority: undefined },
    { id: 'c', priority: 2 },
  ]);

  assert.deepEqual(tiers, [['a', 'b'], ['c']]);
  assertDense(tiers);
});

test('the tier count is the next free dense priority', () => {
  const count = (rows) => countPriorityTiers(rows, (row) => row.priority);

  assert.equal(count([]), 0);
  assert.equal(count([{ priority: 0 }]), 1);
  assert.equal(count([{ priority: 0 }, { priority: 0 }, { priority: 0 }]), 1);
  // Legacy values that are not dense yet still count as one tier each.
  assert.equal(count([{ priority: 10 }, { priority: 50 }]), 2);
  assert.equal(count([{ priority: 0 }, { priority: 1 }, { priority: 2 }]), 3);
});

test('empty tiers are closed up so the first tier is always P0', () => {
  const tiers = [['a'], ['b', 'c']];

  // 'a' was the only rule in P0. Joining the tier below must not leave P0
  // behind as a priority no rule can ever be assigned to again.
  const afterMove = moveRowToTier(tiers, 'a', 1);
  assert.deepEqual(afterMove, [['b', 'c', 'a']]);
  assert.deepEqual(prioritiesOf(afterMove), { a: 0, b: 0, c: 0 });
  assertDense(afterMove);

  // Pulling the only rule out of a tier in the middle closes that tier up too.
  const afterSplit = insertRowAsNewTier([['a'], ['b'], ['c']], 'b', 0);
  assert.deepEqual(afterSplit, [['b'], ['a'], ['c']]);
  assert.deepEqual(prioritiesOf(afterSplit), { a: 1, b: 0, c: 2 });
  assertDense(afterSplit);
});

test('dropping a row onto an existing tier joins it', () => {
  const tiers = [['a'], ['b', 'c']];

  assert.deepEqual(moveRowToTier(tiers, 'a', 1), [['b', 'c', 'a']]);
  assert.deepEqual(prioritiesOf(moveRowToTier(tiers, 'a', 1)), { a: 0, b: 0, c: 0 });
  assert.deepEqual(moveRowToTier(tiers, 'b', 0), [['a', 'b'], ['c']]);
});

test('dropping a row onto its own tier is a no-op', () => {
  const tiers = [['a'], ['c']];

  assert.equal(moveRowToTier(tiers, 'a', 0), tiers);
  assert.deepEqual(insertRowAsNewTier(tiers, 'a', 0), tiers);
});

test('the gap above a shared tier splits the dragged row out in front of it', () => {
  const tiers = [['a', 'b'], ['c']];

  // 'a' leaves the tier it shared with 'b' and becomes a tier of its own above it.
  assert.deepEqual(insertRowAsNewTier(tiers, 'a', 0), [['a'], ['b'], ['c']]);
  assertDense(insertRowAsNewTier(tiers, 'a', 0));
});

test('the last gap always creates a new lowest-priority tier', () => {
  const tiers = [['a', 'b']];

  // Regression: splitting the only tier must not saturate at one priority.
  const split = insertRowAsNewTier(tiers, 'b', 1);
  assert.deepEqual(split, [['a'], ['b']]);
  assert.deepEqual(prioritiesOf(split), { a: 0, b: 1 });
  assertDense(split);

  // And the tier count keeps growing instead of reusing the last priority.
  const grown = insertRowAsNewTier(split, 'a', 2);
  assert.deepEqual(grown, [['b'], ['a']]);
  assert.deepEqual(prioritiesOf(grown), { a: 1, b: 0 });
  assertDense(grown);
});

test('a new tier can be inserted at the head, the middle and the end', () => {
  const tiers = [['a'], ['b'], ['c']];

  assert.deepEqual(insertRowAsNewTier(tiers, 'c', 0), [['c'], ['a'], ['b']]);
  assert.deepEqual(insertRowAsNewTier(tiers, 'c', 1), [['a'], ['c'], ['b']]);
  assert.deepEqual(insertRowAsNewTier(tiers, 'a', 3), [['b'], ['c'], ['a']]);

  for (const result of [insertRowAsNewTier(tiers, 'c', 0), insertRowAsNewTier(tiers, 'c', 1), insertRowAsNewTier(tiers, 'a', 3)]) {
    assertDense(result);
  }
});

test('a gap directly below the row own tier leaves the list untouched', () => {
  const tiers = [['a'], ['b'], ['c']];

  assert.deepEqual(insertRowAsNewTier(tiers, 'a', 1), tiers);
  assert.deepEqual(insertRowAsNewTier(tiers, 'b', 2), tiers);
  assert.deepEqual(insertRowAsNewTier(tiers, 'a', 0), tiers);
});

test('an unknown row id leaves the tiers untouched', () => {
  const tiers = [['a'], ['b']];

  assert.equal(moveRowToTier(tiers, 'missing', 0), tiers);
  assert.equal(insertRowAsNewTier(tiers, 'missing', 1), tiers);
});

test('the dialog and the shared model schema agree on the priority cap', () => {
  const componentsDir = join(import.meta.dirname, '..', 'components');
  const dialog = readFileSync(join(componentsDir, 'models-association-dialog.tsx'), 'utf8');
  const sharedSchema = readFileSync(join(import.meta.dirname, 'schema.ts'), 'utf8');

  assert.match(dialog, /priority: associationPrioritySchema,/);
  assert.match(sharedSchema, new RegExp(`priority: z\\.number\\(\\)\\.min\\(0\\)\\.max\\(${MAX_ASSOCIATION_PRIORITY}\\)`));
  // Priority is derived from the tiers, so the dialog must not edit it directly.
  assert.doesNotMatch(dialog, /nextAssociationPriority|hasInvalidAssociationPriority/);
  assert.match(dialog, /moveRowToTier\(/);
  assert.match(dialog, /insertRowAsNewTier\(/);
});
