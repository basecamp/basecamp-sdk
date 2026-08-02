# frozen_string_literal: true

# Canonicalization shared by generate-bc3-routes and check-bc3-route-parity.
#
# The two sides of the route-parity comparison are written in different
# dialects. bc3's API docs say `GET /inboxes/3/inbox_forwards.json`; the SDK's
# openapi.json says `GET /{accountId}/inboxes/{inboxId}/inbox_forwards.json`.
# Both must reduce to the same string or every route looks like a mismatch.
#
# This file is the single definition of that reduction. It lives here rather
# than inside either script because a normalizer that drifts between the
# generator and the checker produces silent false greens: the generator writes
# routes in dialect A, the checker looks for dialect B, nothing matches, and the
# floor checks are the only thing standing between that and a vacuous pass.
module Bc3RouteNormalizer
  METHODS = %w[GET POST PUT PATCH DELETE].freeze

  # Order matters throughout. Each step assumes the previous ones have run.
  def self.normalize(path)
    p = path.to_s.strip
    p = p.sub(%r{\Ahttps?://[^/]+}, '')          # absolute example URL -> path
    p = p.split('?', 2).first.to_s               # drop query string
    p = p.sub(%r{\A/\{accountId\}}, '')          # SDK account prefix
    p = p.sub(%r{\A/\$ACCOUNT_ID}, '')           # curl-example account var
    p = p.sub(%r{\A/:account_id}, '')            # registry `routes:` dialect
    p = p.sub(/\.json\z/, '')                    # trailing .json

    # Every identifier-shaped segment collapses to :id. Comparing route SHAPE,
    # not parameter names: the SDK calls it {groupId} and bc3 calls it 3, and
    # neither name is evidence about whether the route exists.
    p = p.gsub(/\{[^}]*\}/, ':id')               # {todoId}, {recording_id}
    p = p.gsub(%r{(?<=/)\$[A-Z0-9_]+(?=/|\z)}, ':id')      # $BUCKET_ID
    p = p.gsub(%r{(?<=/):[A-Za-z_][A-Za-z0-9_]*(?=/|\z)}, ':id') # :bucket_id
    p = p.gsub(%r{(?<=/)\d+(?=/|\z)}, ':id')     # literal doc ids: /inboxes/3

    # SGID/base64 bookmark tokens appear as opaque path segments in examples.
    p = p.gsub(%r{(?<=/)[A-Za-z0-9+/=_-]{32,}(?=/|\z)}, ':id')

    p = p.squeeze('/')
    p = p.chomp('/') unless p == '/'
    p.empty? ? '/' : p
  end

  # A route's identity for set comparison.
  def self.key(method, path)
    "#{method.to_s.upcase} #{normalize(path)}"
  end

  # Bucket-insensitive identity — for the bc3 -> SDK direction ONLY.
  #
  # ⚠️ Do NOT use this to accept an SDK route. It was originally used in both
  # directions and that was a defect: it silently accepted 12 SDK operations
  # whose flat spelling bc3 documents only bucket-scoped (five chatbot ops, four
  # client-portal reads, EnableTool/DisableTool/RepositionTool). Four confirmed
  # defects in this SDK are exactly that shape — an invented flat variant of a
  # bucket-only route — so collapsing the distinction defeats the gate's whole
  # purpose on the side where a wrong spelling is a live 404.
  #
  # It is right for the bc3 -> SDK direction, which is a coverage ledger asking
  # "do we model this resource+verb at all". There the spelling is a modeling
  # decision, not a reachability risk. Measured at bc3 d0edc1283b: exact
  # matching in that direction reports 143 unmodeled routes, of which 125 (87%)
  # are the other spelling of a resource the SDK already models. Bucket
  # collapsing takes it to 18 real ones.
  #
  # Note this is also NOT what check-bucket-flat-parity does. That lint covers
  # only `GET`s whose response is a list, and it checks SDK-internal
  # consistency (does a flat counterpart exist in our own spec) — never whether
  # bc3 serves either spelling.
  def self.bucket_insensitive_key(method, path)
    "#{method.to_s.upcase} #{normalize(path).sub(%r{\A/buckets/:id}, '')}"
  end
end
