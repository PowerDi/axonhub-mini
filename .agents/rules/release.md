# Release / Image Publishing Rules

Scope: manual (applies whenever building or publishing a Docker image)

1. Image publishing is manual only: `.github/workflows/docker-ghcr.yml` via `workflow_dispatch`. Never wire it to push events.
2. Version scheme: `internal/build/VERSION` (the upstream release tag actually merged into this fork) + `.github/mini-version.txt` (this fork's own plain-integer counter). A prerelease-based tag looks like `v1.0.0-rc2.mini.26`.
   - When syncing to an upstream release, update `internal/build/VERSION` to that exact upstream tag before publishing. Verify that tag is an ancestor of the release commit; do not substitute the newest unmerged upstream release.
   - Do not assume the upstream source tree has already updated this file: upstream `release.yml` deliberately skips writing it for beta/rc releases. A successful merge therefore does not guarantee the base version is current.
   - Keep the Mini counter monotonically increasing across upstream baseline changes; do not reset it.
   - The workflow writes the final image version into the build's `VERSION` file. Correct a stale base with a new build, not just a registry alias, so the image tag and the server-reported version agree.
3. Before triggering a build, check whether the current counter value has already been published (inspect the latest `docker-ghcr.yml` run). If yes, bump `.github/mini-version.txt` by 1 **first**, in its own commit: `ci(ghcr): bump mini counter to N`. Never rebuild with an already-published counter value — tags collide.
4. Push to `origin` (PowerDi/axonhub-mini), then trigger:
   `gh workflow run docker-ghcr.yml --ref main -f arch=amd64` (arch: amd64 default / arm64 / both; `extra_tag` optional).
5. gh default repo for this checkout is PowerDi/axonhub-mini (`gh repo set-default`). If a gh command 404s, pass `-R PowerDi/axonhub-mini`. NEVER push to the `upstream` remote.
6. Watch the run to a terminal state (`gh run watch --exit-status`); past runs take 3–5 minutes. Report the run URL and the published tag list.
7. Release notes / build summaries go to `.agent/summary/`.
