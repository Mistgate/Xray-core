# Xray-core, with the Vision data race fixed

This is a fork of [XTLS/Xray-core](https://github.com/XTLS/Xray-core): upstream **v26.11.9** (commit `676eaf66f8f0`) plus
**one commit** that fixes a data race in XTLS Vision. Nothing else differs. For everything about Xray (features, configuration,
documentation, releases) see upstream; this page only describes what the fork changes.

Maintained for [Mistgate](https://github.com/Mistgate/mistgate), which builds from it. It is published so that anyone can use or
review the same patch.

## What is changed

`proxy.TrafficState` is shared by the two copy directions of one Vision connection. `VisionReader` and `VisionWriter` run in
different goroutines, and `XtlsFilterTls` is called from both. Six fields were written and read by both without synchronisation:
`NumberOfPacketToFilter`, `RemainingServerHello`, `IsTLS`, `IsTLS12orAbove`, `EnableXtls` and `Cipher`. That is a data race, and
the Go race detector reports it in any test that pushes traffic through Vision. Upstream closed the pull request that fixes it
([XTLS/Xray-core#6706](https://github.com/XTLS/Xray-core/pull/6706)) without merging.

The patch turns the six fields into `atomic.Int32`, `atomic.Bool` and `atomic.Uint32` and routes every access through `Load`,
`Store` or `Add`. Values and the order of operations are unchanged, so the wire behaviour is the same.

The diff against upstream is three files:

- `proxy/proxy.go`: the `TrafficState` fields, `NewTrafficState` (initial values through `Store`: 8 packets to filter,
  `RemainingServerHello` -1) and every use in `VisionReader`, `VisionWriter` and `XtlsFilterTls`.
- `proxy/vision_test.go` (new): `TestVisionTrafficStateRace` drives a reader and a writer over one `TrafficState` from two
  goroutines.
- `README.md`: this page, in place of upstream's.

Nothing outside `proxy/proxy.go` reads these fields.

## Reproduce

```sh
git diff v26.11.9 --stat          # README.md, proxy/proxy.go, proxy/vision_test.go
go test -race -run TestVisionTrafficStateRace ./proxy/
```

The test fails under `-race` on upstream v26.11.9 (copy `proxy/vision_test.go` onto it) and passes on this branch.

## Use it

The module path is unchanged (`github.com/xtls/xray-core`), so a `replace` is all it takes:

```
replace github.com/xtls/xray-core => github.com/Mistgate/Xray-core v0.0.0-<timestamp>-<commit>
```

`go get github.com/Mistgate/Xray-core@mistgate` prints the exact version for the current commit, or
`go mod edit -replace github.com/xtls/xray-core=github.com/Mistgate/Xray-core@<commit>`. Mistgate pins a commit of this branch this
way in its `go.mod`; the Go module proxy and checksum database then serve it like any other module.

## Branch and tag naming

- Branch `mistgate` is the current state: the latest upstream release the fork follows, plus the patch. It is rebased onto each
  new upstream release, so its history is rewritten and it must not be pinned by name.
- Tag `mistgate/v<upstream version>-<n>` (for example `mistgate/v26.11.9-1`) marks one such commit and never moves. The number
  `n` counts rebuilds on the same upstream version (a changed patch). The tag is for people and for finding the commit; a
  `go.mod` still pins the commit, through the pseudo-version.
- Pin a commit, not the branch: after a rebase the old commit stays reachable through its tag.

## Move to a new upstream version

1. Fetch upstream and check whether the race is fixed there (`TrafficState` in `proxy/proxy.go`: atomic or locked fields). If it
   is, retire the fork.
2. Otherwise start a new branch state from the new upstream tag and carry the one commit over: `git rebase --onto <new tag>
   <old tag> mistgate`, or cherry-pick it and redo the hunk by hand if it no longer applies (six fields, `Load`/`Store`/`Add`).
   Keep it a single commit on top of the tag.
3. Update the base version and commit named at the top of this page, run `go build ./... && go vet ./proxy/` and the test above
   with `-race`, and tag the commit `mistgate/v<new version>-1`.
4. In Mistgate, point `replace` at the new commit and raise the `require` of `github.com/xtls/xray-core` to the same upstream
   version; its `internal/vendorcheck` test fails until both agree.

## License

[MPL-2.0](LICENSE), unchanged from upstream. Every source file keeps its upstream header.
