#!/usr/bin/env bash
set -euo pipefail

# Guard: fetchSignedDownload is the hop-2-only primitive for fields tagged
# @basecampAuthRoutableUrl. It must be called only from download.go (via
# fetchAPIDownload, which performs the authenticated hop 1 first). Any
# other caller is either re-inventing the two-hop flow or skipping hop 1.

VIOLATIONS=$(grep -rnE --include='*.go' --exclude='*_test.go' \
  '\bfetchSignedDownload[[:space:]]*\(' go/pkg/basecamp/ \
  | grep -v '/download\.go:' || true)

if [ -n "${VIOLATIONS}" ]; then
  echo "ERROR: fetchSignedDownload call-site outside go/pkg/basecamp/download.go"
  echo ""
  echo "${VIOLATIONS}"
  echo ""
  echo "Consumers of @basecampAuthRoutableUrl fields (e.g., Upload.download_url,"
  echo "CampfireLineAttachment.download_url) MUST route through the two-hop helper"
  echo "Client.fetchAPIDownload (or the public AccountClient.DownloadURL), which"
  echo "performs the authenticated first hop before the signed fetch."
  echo ""
  echo "See spec/basecamp-traits.smithy: basecampAuthRoutableUrl contract."
  exit 1
fi

echo "auth-routable consumer check: passed"
