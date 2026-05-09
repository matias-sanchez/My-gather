#!/bin/sh
# extract-changelog-section.sh — canonical CHANGELOG.md section extractor.
#
# Synopsis:
#   extract-changelog-section.sh <tag> [<changelog-path>]
#
# Prints to stdout the lines of the section whose heading begins with
# "## <tag> (", excluding the heading line itself, terminating
# immediately before the next "## v" heading or at end-of-file.
#
# Single canonical implementation (Principle XIII). Consumed unmodified
# by .github/workflows/ci.yml (publish-release) and by
# tests/release/extract_changelog_section_test.go.
#
# Exit codes:
#   0  success
#   1  no matching "## <tag> (" heading found in the changelog
#   2  usage error or unreadable changelog
set -eu

LC_ALL=C
export LC_ALL

usage() {
	echo "usage: extract-changelog-section.sh <tag> [<changelog-path>]" >&2
}

if [ $# -lt 1 ] || [ -z "$1" ]; then
	usage
	exit 2
fi

TAG=$1
CHANGELOG=${2:-CHANGELOG.md}

if [ ! -r "$CHANGELOG" ]; then
	echo "extract-changelog-section.sh: cannot read $CHANGELOG" >&2
	exit 2
fi

# Verify the heading exists before extracting, so a missing tag is a
# distinct exit code (1) from "found, here's the body" (0).
if ! grep -qF "## ${TAG} (" "$CHANGELOG"; then
	echo "extract-changelog-section.sh: no section for tag ${TAG} in ${CHANGELOG}" >&2
	exit 1
fi

# Extract: starting at the line AFTER the matching heading, print every
# line up to (but not including) the next "## v" heading or end-of-file.
# The "in_section" / "found" two-state machine handles both the
# "section in the middle of the file" and "section is the last in the
# file" cases without a fallback path.
awk -v tag="$TAG" '
BEGIN {
	heading = "## " tag " ("
	in_section = 0
}
{
	if (in_section) {
		if (index($0, "## v") == 1) {
			exit
		}
		print
		next
	}
	if (index($0, heading) == 1) {
		in_section = 1
		next
	}
}
' "$CHANGELOG"
