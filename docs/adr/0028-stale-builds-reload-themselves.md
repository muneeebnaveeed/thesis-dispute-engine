# 0028: A tab that outlived its build reloads itself, once

**Status:** accepted, 2026-09-30; builds on ADR 0020 and 0021

## Context

A browser tab keeps the build that served it. After the workbench is redeployed, that tab still calls server
functions and lazy route chunks by the identities its build knows. Server function ids are hashes of file and export
name, so they survive most deploys, but a moved or renamed function, or any code-split chunk, is gone from the new
server. The server answers such a call with a bare 500 (`{"message":"HTTPError"}`) and a missing chunk with a 500 too,
indistinguishable from a real fault: the analyst sees a generic failure banner, and "Try again" fails the same way
until they reload by hand.

## Decision

Every build is stamped with a version (the commit, `git describe`, through `VITE_BUILD_VERSION` at image build), and
the tab compares its own stamp with the server's when staleness is plausible, reloading once if they differ.

- The server answers `GET /build-version` with its stamp, never cached.
- The tab asks when a server function call fails without a problem body, when a read throws, when a route fails to
  load, when Vite reports a chunk that could not be loaded (`vite:preloadError`), and when the tab becomes visible
  again (at most once a minute).
- A mismatch reloads the page, at most once per server build per tab (`sessionStorage`), so a server that keeps
  disagreeing cannot trap a tab in a loop; a failed chunk reloads at most once per tab build. An unreachable server is
  an outage, not a stale build, and changes nothing. Unstamped (development) builds never check.

## Consequences

- Easier: a redeploy no longer leaves open tabs failing until someone reloads; the check costs nothing on the happy
  path, one tiny request after a failure or on returning to the tab.
- Harder: a reload discards what was typed into an open form, which is the same loss a manual reload causes but now
  happens without asking; one extra public route; the version must be passed into every image build (the Makefile
  and compose do it).
