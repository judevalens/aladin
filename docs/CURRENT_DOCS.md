# Current Documentation Map

Status: cleanup guide as of 2026-08-14.

Use this file to decide which docs to trust when Aladin's older product language
conflicts with the current trading research direction.

## Canonical

- `../CURRENT_PRODUCT.md` — current product truth and cleanup baseline.
- `../README.md` — repository orientation and common commands.
- `../design/TRADING_PRD.md` — active product north star. Product intent is current;
  implementation snapshots inside it may lag the code.
- `../design/UI_ARCHITECTURE.md` — current frontend architecture and UI conventions.

## Current Substrate Docs

- `../backend_v2/PIPELINE.md` — ingestion architecture, but needs a refresh for the
  current entity/embed/optional-graph pipeline.
- `AUTH_SYSTEM.md` — auth design.
- `NANGO_PROVIDER_CONNECTIONS.md` — provider connection design.
- `GLOBAL_SOURCE_ITEM_PIPELINE.md` — source item correctness model.

## Historical Or Parked

- `archive/ALADIN_PRODUCT_VISION.md` — old graph-grounded broad AI workspace thesis.
- `../design/archive/DATA_MODEL.md` — useful entity/bridge thinking, but scoped to the parked
  generic knowledge product.
- `archive/REMAINING_FEATURES_AUDIT.md` — old PRD gap audit for graph/home/signals surfaces.
- `archive/PIPELINE_AUDIT.md` — historical audit from before the current live entity/embed
  pipeline was wired.
- Tutor docs and spikes — design record only unless explicitly revived.

## Rule

New work should cite the canonical docs above. Historical docs can supply context, but
they should not define product direction without first updating `CURRENT_PRODUCT.md`.
