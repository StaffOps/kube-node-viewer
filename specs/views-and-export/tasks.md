# Tasks: views-and-export

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. Excluding filters: `Filter.Exclude`, matching, `x` in the Filters panel, header and panel text
  - _Covers: R1 · Depends on: — · Verified by: unit and key-driven tests_
- [x] T2. Named views: state file `views`, Views popup (`v`) with name input, apply / save / delete
  - _Covers: R2 · Depends on: — · Verified by: state and key-driven tests_
- [ ] T3. Export (`e`): CSV and JSON of the nodes on screen, off the UI goroutine, footer notice
  - _Covers: R3 · Depends on: — · Verified by: unit tests writing to a temp dir_
- [ ] T4. README (keys, files), CHANGELOG, screenshots
  - _Covers: — · Depends on: T1–T3 · Verified by: review_

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Typed filter syntax in `/` | panels feel slow for frequent filters |
| Choosing the export directory | users run it from read-only directories |
