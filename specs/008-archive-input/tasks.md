# Tasks: Archive Input

**Input**: Design documents from `specs/008-archive-input/`  
**Prerequisites**: spec.md, plan.md, research.md, data-model.md, contracts/cli.md, quickstart.md

**Tests**: Included because archive input changes CLI behavior and untrusted
input handling.

## Phase 1: Setup

- [x] T001 Create feature branch and Spec Kit directory for archive input.
- [x] T002 Record incident inventory counts and evidence paths in `research.md`.
- [x] T003 Define archive input CLI contract in `contracts/cli.md`.

## Phase 2: Foundation

- [x] T004 Add cheap pt-stalk root recognition helper in `parse/parse.go`.
- [x] T005 Add CLI archive input resolver in `cmd/my-gather/archive_input.go`.
- [x] T006 Map archive input errors to existing exit codes in `cmd/my-gather/main.go`.
- [x] T007 Update help and positional wording from `<input-dir>` to `<input>`.

## Phase 3: Safe Archive Extraction

- [x] T008 Support `.zip` extraction with traversal and special-entry rejection.
- [x] T009 Support `.tar`, `.tar.gz`, and `.tgz` extraction.
- [x] T010 Support `.gz` extraction, including content detection for tar streams.
- [x] T011 Enforce extracted-byte bounds during extraction.
- [x] T012 Remove temporary extraction directories on success and failure.

## Phase 4: Tests

- [x] T013 Add CLI test for nested zip archive input in `cmd/my-gather/main_test.go`.
- [x] T014 Add CLI test for nested tar-gzip archive input in `cmd/my-gather/main_test.go`.
- [x] T015 Add CLI test for archive path traversal rejection in `cmd/my-gather/main_test.go`.
- [x] T016 Run focused validation from `quickstart.md`.

## Phase 5: Documentation And Final Validation

- [x] T017 Update feature docs and active Spec Kit pointers.
- [x] T018 Run full repository validation with `go test ./...`.
- [x] T019 Generate a real report from a local incident archive and verify output.
