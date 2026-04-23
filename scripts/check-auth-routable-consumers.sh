#!/usr/bin/env bash
set -euo pipefail

# Guard: fetchSignedDownload is the hop-2-only primitive for fields tagged
# @basecampAuthRoutableUrl. It must be called only from download.go, and only
# from the single call site inside fetchAPIDownload that performs the
# authenticated hop 1 first. Any other caller is either re-inventing the
# two-hop flow or skipping hop 1.

# Rule 1: any reference to fetchSignedDownload( in non-test Go code outside
# download.go is a violation. The broad `\b...\(` pattern intentionally also
# matches doc-comment examples — stale examples warrant review.
EXTERNAL=$(grep -rnE --include='*.go' --exclude='*_test.go' \
  '\bfetchSignedDownload[[:space:]]*\(' go/pkg/basecamp/ \
  | grep -v '^go/pkg/basecamp/download\.go:' || true)

if [ -n "${EXTERNAL}" ]; then
  echo "ERROR: fetchSignedDownload reference outside go/pkg/basecamp/download.go"
  echo ""
  echo "${EXTERNAL}"
  echo ""
  echo "Consumers of @basecampAuthRoutableUrl fields (e.g., Upload.download_url,"
  echo "CampfireLineAttachment.download_url) MUST route through the two-hop helper"
  echo "Client.fetchAPIDownload (or the public AccountClient.DownloadURL), which"
  echo "performs the authenticated first hop before the signed fetch."
  echo ""
  echo "See spec/basecamp-traits.smithy: basecampAuthRoutableUrl contract."
  exit 1
fi

# Rule 2: download.go must contain exactly one method-call site of
# fetchSignedDownload — the one inside fetchAPIDownload. The function
# declaration `func (c *Client) fetchSignedDownload(` lacks the leading `.`
# and is excluded by this pattern. A second call site in download.go would
# silently bypass Rule 1's file-level exemption, so this rule surfaces it.
CALL_SITES=$(grep -cE '\.fetchSignedDownload[[:space:]]*\(' go/pkg/basecamp/download.go || true)

if [ "${CALL_SITES}" != "1" ]; then
  echo "ERROR: download.go has ${CALL_SITES} call site(s) of fetchSignedDownload, expected exactly 1"
  echo ""
  grep -nE '\.fetchSignedDownload[[:space:]]*\(' go/pkg/basecamp/download.go || true
  echo ""
  echo "The only legitimate call is inside Client.fetchAPIDownload, after the"
  echo "authenticated first hop. If you are adding a genuinely new caller,"
  echo "update this guard to reflect the new invariant."
  exit 1
fi

echo "auth-routable consumer check: passed"
