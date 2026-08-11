# Cluster Flow Shell Design

## Goal

Make the default Orchestrator experience coherent from its landing route through cluster exploration. `/web/clusters` becomes a concise operational landing page; cluster detail uses the same compact frame and gives topology priority.

## Scope

- Redesign `/web/clusters` as the primary cluster landing page.
- Flatten the cluster-detail header into a single compact identity row.
- Replace the visually dominant left command rail with a small inline View/control menu beside cluster identity.
- Keep the current semantic topology cards and D3 renderer.

## Information Architecture

`global navigation → cluster landing list → selected cluster identity → topology canvas`

The global navigation stays a compact dark bar. The landing page shows one row/card per cluster with name, alias, primary, member count, health/problem summary, and an explicit open action. Selecting a cluster opens its existing detail route.

The cluster page starts with an identity row: cluster name, primary, member/health summary, and a compact View menu. The topology canvas follows immediately. The command controls retain their existing data-command values and behavior but no longer define the page’s visual hierarchy.

## Visual Direction

- Use the dark bar only for global application context.
- Use a warm white/light-gray content surface for landing and topology work.
- Prefer dense, legible operational text over dashboard decoration.
- Reserve warning/error color and emphasis for real topology state.
- Keep controls labelled, keyboard reachable, and grouped by purpose.

## Constraints

- Preserve routes, APIs, recovery/failover, drag/drop, D3 positioning, and node modal behavior.
- Preserve existing cluster commands while changing their placement/presentation.
- Keep page-specific CSS scoped; do not extend global Bootstrap compatibility shims.
- The `/` redirect remains `/web/clusters`; the redesigned landing must therefore stand on its own.

## Verification

- Template tests protect the landing-page landmarks and retained command hooks.
- Functional smoke checks confirm `/`, `/web/clusters`, and a live cluster detail route return the intended page shells.
- Manual lab check confirms the landing page is readable, the selected cluster opens, view controls work, and topology remains interactable.

