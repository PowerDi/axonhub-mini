import { z } from 'zod';

// Priorities are a dense tier index derived from list position: the first tier
// is 0, every tier after it is one higher, and no tier is ever empty. Rules that
// share a tier are load-balanced against each other, and tiers are tried in
// order, so the tier index doubles as the failover order.
//
// The bound below only mirrors the shared modelAssociationSchema (0-100) and the
// GraphQL Int field it is serialized into; the dialog can never build more tiers
// than the ten rules a model is allowed to have.
export const MAX_ASSOCIATION_PRIORITY = 100;

export const associationPrioritySchema = z
  .number({ error: 'Priority is required' })
  .int('Priority must be an integer')
  .min(0, 'Priority must be at least 0')
  .max(MAX_ASSOCIATION_PRIORITY, `Priority cannot exceed ${MAX_ASSOCIATION_PRIORITY}`);

/** Row ids grouped by tier, ordered from the first tier tried to the last one. */
export type PriorityTiers = string[][];

// Priorities only change through the helpers below, so a missing or out-of-range
// value can only come from legacy data. Folding it back into range keeps the
// grouping total instead of dropping the row from every tier.
function normalizePriority(priority: number | null | undefined): number {
  if (priority === null || priority === undefined || !Number.isFinite(priority)) {
    return 0;
  }

  return Math.min(Math.max(Math.floor(priority), 0), MAX_ASSOCIATION_PRIORITY);
}

/**
 * Groups rows into tiers by their priority. Tiers come back in ascending
 * priority order and empty tiers never appear, so the index of a tier in the
 * result is exactly the dense priority it should be renumbered to.
 */
export function buildPriorityTiers<T>(
  rows: readonly T[],
  rowID: (row: T) => string,
  priorityOf: (row: T) => number | null | undefined
): PriorityTiers {
  const buckets = new Map<number, string[]>();

  for (const row of rows) {
    const priority = normalizePriority(priorityOf(row));
    const bucket = buckets.get(priority);

    if (bucket) {
      bucket.push(rowID(row));
    } else {
      buckets.set(priority, [rowID(row)]);
    }
  }

  return [...buckets.entries()]
    .sort(([left], [right]) => left - right)
    .map(([, ids]) => ids);
}

/** Maps every row id to the dense index of the tier it ended up in. */
export function priorityByRow(tiers: PriorityTiers): Map<string, number> {
  const priorities = new Map<string, number>();

  tiers.forEach((tier, priority) => {
    for (const rowID of tier) {
      priorities.set(rowID, priority);
    }
  });

  return priorities;
}

/**
 * Number of distinct tiers. Because tiers are numbered 0..n-1, this is also the
 * dense priority a rule appended as a new last tier should get.
 */
export function countPriorityTiers<T>(rows: readonly T[], priorityOf: (row: T) => number | null | undefined): number {
  return new Set(rows.map((row) => normalizePriority(priorityOf(row)))).size;
}

/**
 * Moves a row into an existing tier. Dropping a row back onto the tier it
 * already belongs to changes nothing, and a tier that the move empties is
 * dropped, which is what keeps the remaining numbering dense.
 */
export function moveRowToTier(tiers: PriorityTiers, rowID: string, targetTier: number): PriorityTiers {
  const sourceTier = tiers.findIndex((tier) => tier.includes(rowID));

  if (sourceTier === -1 || sourceTier === targetTier) {
    return tiers;
  }

  const { remaining, sourceDropped } = removeRow(tiers, rowID);
  // Dropping an emptied tier shifts every tier after it down by one.
  const index = sourceDropped && sourceTier < targetTier ? targetTier - 1 : targetTier;

  if (index < 0 || index >= remaining.length) {
    return tiers;
  }

  const next = remaining.map((tier) => [...tier]);
  next[index].push(rowID);

  return next;
}

/**
 * Moves a row out into a tier of its own. Gaps are the insertion slots between
 * tiers, indexed `0..tiers.length`, so the last gap always grows a brand new
 * lowest-priority tier instead of saturating an existing one.
 */
export function insertRowAsNewTier(tiers: PriorityTiers, rowID: string, gapIndex: number): PriorityTiers {
  const sourceTier = tiers.findIndex((tier) => tier.includes(rowID));

  if (sourceTier === -1 || gapIndex < 0 || gapIndex > tiers.length) {
    return tiers;
  }

  const { remaining, sourceDropped } = removeRow(tiers, rowID);
  const index = sourceDropped && sourceTier < gapIndex ? gapIndex - 1 : gapIndex;
  const next = remaining.map((tier) => [...tier]);
  next.splice(index, 0, [rowID]);

  return next;
}

function removeRow(tiers: PriorityTiers, rowID: string): { remaining: PriorityTiers; sourceDropped: boolean } {
  const remaining = tiers.map((tier) => tier.filter((id) => id !== rowID)).filter((tier) => tier.length > 0);

  return { remaining, sourceDropped: remaining.length < tiers.length };
}
