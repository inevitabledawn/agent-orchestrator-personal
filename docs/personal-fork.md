# Personal fork source

The maintained source is the fork's default branch (initially
`personal/maintained`), a feature stack over the
pinned upstream commit in [`fork/series.json`](../fork/series.json). The desktop
revision is recorded in `frontend/package.json` and must be reviewed on each
upstream update. Do not build the archived `personal/v0.13.1`
snapshot when intending to use the maintained source.

See [`fork/README.md`](../fork/README.md) for patch boundaries, reproduction,
safe upstream updates, database migration collisions, and contribution branches.
The end of the top-level README lists implemented Personal behavior.

No app installation, production data migration, or automatic updater change is
performed by the patch tooling. Keep automatic replacement with official
upstream binaries disabled when packaging a Personal build. App/runtime state,
credentials, and database snapshots do not belong in this repository.
