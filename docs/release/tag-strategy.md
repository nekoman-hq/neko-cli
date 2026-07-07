# Tag Strategy

Each normalized release unit has a tag prefix and a `TagSpec`.

`TagSpec` provides:

- `Format(version)` to produce a tag.
- `Parse(tag)` to extract a version only when the tag exactly belongs to the unit.
- `Matches(tag)` for exact ownership checks.
- `Pattern()` for coarse Git tag prefiltering.

Supported prefix examples:

```text
v
api/v
web/v
mobile/v
```

Examples:

```text
v2.2.4           -> v + 2.2.4
api/v0.1.0       -> api/v + 0.1.0
web/v1.4.2-rc.1  -> web/v + 1.4.2-rc.1
```

A tag matches only when:

- it starts exactly with the unit prefix;
- the remaining text is a valid version accepted by the release tag parser;
- the remaining text contains no slash or unrelated suffix.

The V2 validator rejects overlapping prefixes, such as:

```text
api/
api/v
```

That keeps tag ownership unambiguous.

V2 tag lookup is local and read-only. It does not run `git fetch`. Git glob patterns are used only as a coarse prefilter; every returned tag is then checked with `TagSpec.Parse`.
