# Tasks

## 1. Regression test for fixed cell order

- [x] 1.1 Add `TestTabHeader_FixedCellOrder` in `view_test.go` that renders the row for both `tabMessage` and `tabFiles` and asserts `Message` appears before `Files` (by index in the ANSI-stripped line) in both states; verify it fails on current code by running `go test ./... -run TestTabHeader_FixedCellOrder`

## 2. Fix tabHeader() to render in canonical order

- [x] 2.1 Change `model.tabHeader()` to always render `Message` in the first slot and `Files` in the second slot, picking `activeTabStyle` for the slot whose label matches `m.tab`; verify `go test ./...` is green and `TestTabHeader_FixedCellOrder` now passes