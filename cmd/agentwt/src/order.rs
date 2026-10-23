//! Pairwise conflict matrix over agent branch tips (or dirty snapshots) and a simulated merge train.
//!
//! Everything merges in memory on the shared handle. Pairs whose touched-path sets are disjoint never merge.
//! The recommended order per connected component is a maximum independent set of the conflict graph first
//! (exact up to 16 nodes, greedy above), then the rest; one train simulation verifies the order and records the
//! blocking pairs the matrix cannot see (three-way interactions).

use std::collections::{BTreeMap, BTreeSet};

use gix::ObjectId;
use gix::bstr::{BString, ByteSlice};
use gix::merge::tree::TreatAsUnresolved;

use crate::error::{GitExt, Result};
use crate::gitstate::Context;
use crate::wire::{ConflictPair, OrderSummary, RowOrder, TrainOutcome};

/// Exact independent-set search bound (2^16 masks).
const EXACT_BOUND: usize = 16;

/// One branch entering the order computation.
#[derive(Debug, Clone)]
pub struct Input {
    /// Branch name.
    pub name: String,
    /// What is merged: the tip, or the dirty snapshot commit when the worktree had tracked changes.
    pub merge_head: ObjectId,
    /// `merge_head` is a snapshot.
    pub snapshot: bool,
    /// Paths changed vs the default branch's merge-base (prefilter).
    pub touched: BTreeSet<BString>,
}

/// The computed order.
#[derive(Debug, Clone)]
pub struct Order {
    /// Document-level summary.
    pub summary: OrderSummary,
    /// Per-branch facts.
    pub rows: BTreeMap<String, RowOrder>,
}

/// Computes matrix, components, order and train. `default_tip` is the train's starting point.
pub fn compute(context: &Context, default_tip: ObjectId, inputs: &[Input]) -> Result<Order> {
    let n = inputs.len();
    let mut adjacency = vec![BTreeSet::new(); n];
    let mut pairs = Vec::new();
    for i in 0..n {
        for j in (i + 1)..n {
            if inputs[i].touched.is_disjoint(&inputs[j].touched) {
                continue;
            }
            let paths = unresolved_paths(context, inputs[i].merge_head, inputs[j].merge_head)?;
            if paths.is_empty() {
                continue;
            }
            adjacency[i].insert(j);
            adjacency[j].insert(i);
            pairs.push(ConflictPair {
                a: inputs[i].name.clone(),
                b: inputs[j].name.clone(),
                paths,
            });
        }
    }

    let components = connected_components(n, &adjacency);
    let mut train_indexes = Vec::with_capacity(n);
    for component in &components {
        train_indexes.extend(order_component(component, &adjacency));
    }

    let mut rows = BTreeMap::new();
    let mut head = default_tip;
    let mut merged_names = Vec::new();
    for (position, &index) in train_indexes.iter().enumerate() {
        let input = &inputs[index];
        let outcome = match unresolved_paths(context, head, input.merge_head)? {
            paths if paths.is_empty() => {
                head = synthetic_merge(context, head, input.merge_head)?;
                merged_names.push(input.name.clone());
                TrainOutcome::Clean
            }
            paths => TrainOutcome::Blocked {
                blocked_by: merged_names.clone(),
                paths,
            },
        };
        let conflicts_with = adjacency[index]
            .iter()
            .map(|&j| inputs[j].name.clone())
            .collect();
        rows.insert(
            input.name.clone(),
            RowOrder {
                conflicts_with,
                train_position: position + 1,
                train: outcome,
            },
        );
    }

    let summary = OrderSummary {
        train: train_indexes
            .iter()
            .map(|&i| inputs[i].name.clone())
            .collect(),
        pairs,
        components: components
            .iter()
            .map(|component| component.iter().map(|&i| inputs[i].name.clone()).collect())
            .collect(),
        dirty_snapshots: inputs
            .iter()
            .filter(|input| input.snapshot)
            .map(|input| input.name.clone())
            .collect(),
    };
    Ok(Order { summary, rows })
}

/// Unresolved conflict paths of merging `theirs` into `ours` (empty = clean), cheap-probe mode.
fn unresolved_paths(context: &Context, ours: ObjectId, theirs: ObjectId) -> Result<Vec<String>> {
    let repo = context.repo();
    let options = repo
        .tree_merge_options()
        .git()?
        .with_fail_on_conflict(Some(TreatAsUnresolved::git()));
    let labels = gix::merge::blob::builtin_driver::text::Labels::default();
    let outcome = repo
        .merge_commits(ours, theirs, labels, options.into())
        .git()?;
    let mut paths: Vec<String> = outcome
        .tree_merge
        .conflicts
        .iter()
        .filter(|conflict| conflict.is_unresolved(TreatAsUnresolved::git()))
        .map(|conflict| {
            conflict
                .changes_in_resolution()
                .0
                .location()
                .to_str_lossy()
                .into_owned()
        })
        .collect();
    paths.sort();
    paths.dedup();
    Ok(paths)
}

/// Writes the merged tree and a synthetic two-parent commit into object memory; returns its id.
fn synthetic_merge(context: &Context, ours: ObjectId, theirs: ObjectId) -> Result<ObjectId> {
    let repo = context.repo();
    let options = repo.tree_merge_options().git()?;
    let labels = gix::merge::blob::builtin_driver::text::Labels::default();
    let mut outcome = repo
        .merge_commits(ours, theirs, labels, options.into())
        .git()?;
    let tree = outcome.tree_merge.tree.write().git()?.detach();
    let signature = gix::actor::SignatureRef {
        name: b"agentwt".as_bstr(),
        email: b"agentwt@local".as_bstr(),
        time: "0 +0000",
    };
    let commit = repo
        .new_commit_as(
            signature,
            signature,
            "agentwt train step",
            tree,
            [ours, theirs],
        )
        .git()?;
    Ok(commit.id)
}

/// Union-find root of `i` with path compression.
fn find(parent: &mut [usize], i: usize) -> usize {
    let mut root = i;
    while parent[root] != root {
        root = parent[root];
    }
    let mut cursor = i;
    while parent[cursor] != root {
        let next = parent[cursor];
        parent[cursor] = root;
        cursor = next;
    }
    root
}

/// Union-find over the adjacency; components sorted by size descending, then by lowest member index.
fn connected_components(n: usize, adjacency: &[BTreeSet<usize>]) -> Vec<Vec<usize>> {
    let mut parent: Vec<usize> = (0..n).collect();
    for (i, neighbours) in adjacency.iter().enumerate() {
        for &j in neighbours {
            let (a, b) = (find(&mut parent, i), find(&mut parent, j));
            if a != b {
                parent[a] = b;
            }
        }
    }
    let mut groups: BTreeMap<usize, Vec<usize>> = BTreeMap::new();
    for i in 0..n {
        let root = find(&mut parent, i);
        groups.entry(root).or_default().push(i);
    }
    let mut components: Vec<Vec<usize>> = groups.into_values().collect();
    components.sort_by_key(|component| (std::cmp::Reverse(component.len()), component[0]));
    components
}

/// Independent set first (input order), then the rest by ascending degree.
fn order_component(component: &[usize], adjacency: &[BTreeSet<usize>]) -> Vec<usize> {
    let chosen = if component.len() <= EXACT_BOUND {
        exact_independent_set(component, adjacency)
    } else {
        greedy_independent_set(component, adjacency)
    };
    let mut rest: Vec<usize> = component
        .iter()
        .copied()
        .filter(|i| !chosen.contains(i))
        .collect();
    rest.sort_by_key(|&i| (adjacency[i].len(), i));
    let mut order: Vec<usize> = component
        .iter()
        .copied()
        .filter(|i| chosen.contains(i))
        .collect();
    order.extend(rest);
    order
}

fn exact_independent_set(component: &[usize], adjacency: &[BTreeSet<usize>]) -> BTreeSet<usize> {
    let n = component.len();
    let mut best_mask: u32 = 0;
    for mask in 0..(1_u32 << n) {
        if mask.count_ones() <= best_mask.count_ones() {
            continue;
        }
        let independent = (0..n).all(|a| {
            mask & (1 << a) == 0
                || (0..n).all(|b| {
                    mask & (1 << b) == 0 || !adjacency[component[a]].contains(&component[b])
                })
        });
        if independent {
            best_mask = mask;
        }
    }
    (0..n)
        .filter(|a| best_mask & (1 << a) != 0)
        .map(|a| component[a])
        .collect()
}

fn greedy_independent_set(component: &[usize], adjacency: &[BTreeSet<usize>]) -> BTreeSet<usize> {
    let mut by_degree: Vec<usize> = component.to_vec();
    by_degree.sort_by_key(|&i| (adjacency[i].len(), i));
    let mut chosen = BTreeSet::new();
    for i in by_degree {
        if adjacency[i].iter().all(|j| !chosen.contains(j)) {
            chosen.insert(i);
        }
    }
    chosen
}

#[cfg(test)]
mod tests {
    use super::*;

    fn adjacency(n: usize, edges: &[(usize, usize)]) -> Vec<BTreeSet<usize>> {
        let mut adjacency = vec![BTreeSet::new(); n];
        for &(a, b) in edges {
            adjacency[a].insert(b);
            adjacency[b].insert(a);
        }
        adjacency
    }

    #[test]
    fn components_and_exact_set() {
        let adjacency = adjacency(5, &[(0, 1), (1, 2), (3, 4)]);
        let components = connected_components(5, &adjacency);
        assert_eq!(components, vec![vec![0, 1, 2], vec![3, 4]]);
        // Path 0-1-2: independent set {0, 2}; then 1 last.
        assert_eq!(order_component(&components[0], &adjacency), vec![0, 2, 1]);
        assert_eq!(order_component(&components[1], &adjacency), vec![3, 4]);
    }

    #[test]
    fn greedy_matches_exact_on_small_graphs() {
        let cases: [&[(usize, usize)]; 3] =
            [&[], &[(0, 1), (1, 2), (2, 3)], &[(0, 1), (0, 2), (0, 3)]];
        for edges in cases {
            let adjacency = adjacency(4, edges);
            let component: Vec<usize> = (0..4).collect();
            let exact = exact_independent_set(&component, &adjacency);
            let greedy = greedy_independent_set(&component, &adjacency);
            assert_eq!(exact.len(), greedy.len(), "{edges:?}");
        }
    }
}
