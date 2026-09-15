# frozen_string_literal: true

require "test_helper"
require "benchmark"

# Tests for Basecamp::Mentions — the read/write pair over <bc-attachment> sgids.
#
# The sgids here are synthetic: real Rails envelopes with the payload BC3 would
# mint, and a made-up signature. Nothing verifies the signature (nothing can,
# outside BC3), so the digest half only has to be present and base64url-shaped.
class MentionsTest < Minitest::Test
  include TestHelper

  # Builds the older Rails Marshal envelope BC3 serves today:
  # {"gid" => …, "purpose" => …, "expires_at" => nil}.
  def marshal_sgid(gid, purpose: "attachable", signature: "abc123")
    payload = "\x04\x08{\bI\"\bgid\x06:\x06ET" + marshal_string(gid) +
              "I\"\fpurpose\x06;\x00T" + marshal_string(purpose) +
              "I\"\x0Fexpires_at\x06;\x00T0"
    encode(payload, signature)
  end

  # Builds the current Rails envelope: {"_rails" => {"data" => …, "pur" => …}}.
  def rails_sgid(gid, purpose: "attachable", signature: "abc123")
    inner = "{\aI\"\tdata\x06:\x06ET" + marshal_string(gid) + "I\"\bpur\x06;\x00T" + marshal_string(purpose)
    payload = "\x04\x08{\x06I\"\v_rails\x06:\x06ET" + inner
    encode(payload, signature)
  end

  # Builds the JSON spelling Rails' JSON message serializer emits.
  def json_sgid(gid, purpose: "attachable", signature: "abc123")
    encode({ "_rails" => { "data" => gid, "pur" => purpose } }.to_json, signature)
  end

  def marshal_string(value)
    bytes = value.b
    "I\"" + marshal_length(bytes.bytesize) + bytes + "\x06:\x06ET"
  end

  # Marshal's packed integer, for the lengths these fixtures need.
  def marshal_length(length)
    return (length + 5).chr if length < 123

    length < 256 ? "\x01" + length.chr : "\x02" + [ length ].pack("v")
  end

  def encode(payload, signature)
    "#{[ payload.b ].pack("m0").tr("+/", "-_").delete("=")}--#{signature}"
  end

  def person_sgid(id = 1049715915, **)
    marshal_sgid("gid://bc3/Person/#{id}?expires_in", **)
  end

  def test_decodes_a_person_id_from_both_marshal_layouts_and_json
    assert_equal 1049715915, Basecamp::Mentions.person_id_from_sgid(person_sgid)
    assert_equal 42, Basecamp::Mentions.person_id_from_sgid(rails_sgid("gid://bc3/Person/42"))
    assert_equal 42, Basecamp::Mentions.person_id_from_sgid(json_sgid("gid://bc3/Person/42"))
  end

  def test_the_separator_is_the_last_one
    # "-" is a base64url character, so a payload can in principle hold "--";
    # the separator is therefore the LAST one, and the whole value is tried as a
    # bare payload when that split does not decode. The one shape this cannot
    # split is a DIGEST containing "--", which Rails' hex digests never produce
    # — pinned here so the boundary is stated rather than discovered.
    assert_equal 1049715915, Basecamp::Mentions.person_id_from_sgid(person_sgid(1049715915, signature: "de-adbeef"))
    assert_nil Basecamp::Mentions.person_id_from_sgid(person_sgid(1049715915, signature: "de--adbeef"))
  end

  def test_an_unsigned_envelope_decodes
    unsigned = person_sgid.split("--").first

    assert_equal 1049715915, Basecamp::Mentions.person_id_from_sgid(unsigned)
  end

  def test_refuses_a_non_person_gid
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/ActiveStorage::Blob/9"))
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/Recording/9"))
  end

  def test_refuses_a_purpose_other_than_attachable
    # BC3 refuses any other purpose in rich text, so a Person sgid minted for
    # bookmarking is not a mention however valid its gid.
    assert_nil Basecamp::Mentions.person_id_from_sgid(person_sgid(7, purpose: "readable"))
    assert_nil Basecamp::Mentions.person_id_from_sgid(rails_sgid("gid://bc3/Person/7", purpose: "readable"))
  end

  def test_refuses_a_person_gid_that_merely_appears_inside_another_value
    # The envelope is decoded structurally, never searched as bytes.
    assert_nil Basecamp::Mentions.person_id_from_sgid(
      marshal_sgid("gid://bc3/Document/gid://bc3/Person/42")
    )
    assert_nil Basecamp::Mentions.person_id_from_sgid(
      marshal_sgid("gid://bc3/Recording/1", purpose: "gid://bc3/Person/42")
    )
  end

  def test_refuses_a_malformed_path_or_id
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/Person/42/extra"))
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/Person/notanumber"))
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/Person/0"))
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/Person/"))
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("/Person/42"))
  end

  def test_refuses_undecodable_and_oversized_sgids
    assert_nil Basecamp::Mentions.person_id_from_sgid(nil)
    assert_nil Basecamp::Mentions.person_id_from_sgid("")
    assert_nil Basecamp::Mentions.person_id_from_sgid("not base64 at all!!")
    assert_nil Basecamp::Mentions.person_id_from_sgid("#{"A" * 6000}--sig")
  end

  def test_never_instantiates_objects_out_of_a_marshal_payload
    # Marshal.load on an sgid would run arbitrary code out of rich text other
    # people wrote. The restricted reader models six types and nothing else.
    hostile = encode(Marshal.dump({ "gid" => /a regexp is outside the subset/, "purpose" => "attachable" }), "sig")

    assert_nil Basecamp::Mentions.person_id_from_sgid(hostile)
  end

  def test_reads_mentions_in_document_order_without_repeats
    content = <<~HTML
      <div><bc-attachment sgid="#{person_sgid(2)}"></bc-attachment>
      <bc-attachment sgid="#{person_sgid(1)}"></bc-attachment>
      <bc-attachment sgid="#{person_sgid(2, signature: "different")}"></bc-attachment></div>
    HTML

    assert_equal [ 2, 1 ], Basecamp::Mentions.mentioned_person_ids(content)
  end

  def test_counts_a_quoted_mention
    # BC3 notifies quoted mentions too, so the read matches what the server
    # does with the write.
    content = %(<blockquote><bc-attachment sgid="#{person_sgid(3)}"></bc-attachment></blockquote>)

    assert_equal [ 3 ], Basecamp::Mentions.mentioned_person_ids(content)
  end

  def test_skips_attachments_that_are_not_mentions
    content = %(<div><bc-attachment sgid="#{marshal_sgid("gid://bc3/ActiveStorage::Blob/9")}" ) +
              %(caption="a file"></bc-attachment>no mentions here</div>)

    assert_empty Basecamp::Mentions.mentioned_person_ids(content)
  end

  def test_ignores_a_bc_attachment_inside_a_comment_or_another_tags_attribute
    commented = %(<div><!-- <bc-attachment sgid="#{person_sgid(4)}"></bc-attachment> --></div>)
    quoted = %(<div title="<bc-attachment sgid='#{person_sgid(4)}'>">text</div>)

    assert_empty Basecamp::Mentions.mentioned_person_ids(commented)
    assert_empty Basecamp::Mentions.mentioned_person_ids(quoted)
  end

  def test_tag_names_are_matched_whole_and_case_insensitively
    assert_equal [ 5 ], Basecamp::Mentions.mentioned_person_ids(
      %(<BC-Attachment SGID="#{person_sgid(5)}"></BC-Attachment>)
    )
    assert_empty Basecamp::Mentions.mentioned_person_ids(
      %(<bc-attachment-preview sgid="#{person_sgid(5)}"></bc-attachment-preview>)
    )
  end

  def test_attribute_tokenizing_survives_quotes_and_lookalikes
    sgid = person_sgid(6)

    # A ">" inside a quoted value does not end the tag.
    assert_equal [ 6 ], Basecamp::Mentions.mentioned_person_ids(
      %(<bc-attachment caption="a > b" sgid="#{sgid}"></bc-attachment>)
    )
    # An "sgid=" inside another attribute's value is not an attribute.
    assert_empty Basecamp::Mentions.mentioned_person_ids(
      %(<bc-attachment caption="sgid=#{sgid}"></bc-attachment>)
    )
    # The first sgid attribute wins, as HTML resolves a repeated attribute.
    assert_equal [ 6 ], Basecamp::Mentions.mentioned_person_ids(
      %(<bc-attachment sgid='#{sgid}' sgid="#{person_sgid(7)}"></bc-attachment>)
    )
  end

  def test_entity_escapes_in_an_sgid_value_are_decoded
    sgid = person_sgid(8)
    escaped = sgid.sub("--", "&#45;&#45;")

    assert_equal [ 8 ], Basecamp::Mentions.mentioned_person_ids(%(<bc-attachment sgid="#{escaped}"></bc-attachment>))
  end

  def test_an_unterminated_tag_ends_the_walk
    sgid = person_sgid(9)
    content = %(<bc-attachment sgid="#{sgid}"></bc-attachment><bc-attachment sgid=")

    assert_equal [ 9 ], Basecamp::Mentions.mentioned_person_ids(content)
  end

  def test_entity_references_that_matter_to_a_base64_payload_are_decoded
    # A reference expanding to a base64url character changes whether the sgid
    # decodes, not merely how it reads, so the decoder has to cover those.
    sgid = person_sgid(20)
    escaped = sgid.gsub("_", "&lowbar;").gsub("-", "&#45;")

    assert_equal [ 20 ], Basecamp::Mentions.mentioned_person_ids(
      %(<bc-attachment sgid="#{escaped}"></bc-attachment>)
    )
  end

  def test_entity_decoding_is_a_single_pass
    # An escaped reference decodes to the LITERAL reference, never twice. A
    # second pass here would turn "&#66;" back into the "B" the payload needs
    # and resolve a person; one pass leaves a value no base64 alphabet accepts.
    sgid = person_sgid(21)
    doubly = "&amp;#66;#{sgid[1..]}"

    assert_equal "B", sgid[0]
    assert_empty Basecamp::Mentions.mentioned_person_ids(%(<bc-attachment sgid="#{doubly}"></bc-attachment>))
    # And the single-pass form of the same escape does resolve, so the
    # assertion above is about the double decode and not about the payload.
    assert_equal [ 21 ], Basecamp::Mentions.mentioned_person_ids(
      %(<bc-attachment sgid="&#66;#{sgid[1..]}"></bc-attachment>)
    )
  end

  def test_a_line_broken_sgid_still_decodes
    # A line break inside an attribute value is legal HTML, and a base64
    # decoder skips CR and LF — those two only. Indentation is NOT skipped, and
    # is refused here exactly as it is by the reference implementation.
    sgid = person_sgid(22)

    assert_equal [ 22 ], Basecamp::Mentions.mentioned_person_ids(
      %(<bc-attachment\n  sgid="#{sgid[0, 20]}\r\n#{sgid[20..]}"></bc-attachment>)
    )
    assert_empty Basecamp::Mentions.mentioned_person_ids(
      %(<bc-attachment sgid="#{sgid[0, 20]}\n  #{sgid[20..]}"></bc-attachment>)
    )
  end

  def test_a_person_id_past_the_64_bit_range_is_refused
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/Person/99999999999999999999999"))
  end

  def test_a_percent_escaped_gid_path_decodes
    assert_equal 12, Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/Pe%72son/12"))
    assert_equal 12, Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/Person/%31%32"))
  end

  def test_the_walker_is_linear_in_multibyte_text
    # Character indexing would make this quadratic, and the text is whatever
    # somebody else typed. Compared against the same document in pure ASCII so
    # the bound is relative rather than a wall-clock guess.
    tags = %(<bc-attachment sgid="#{person_sgid(23)}"></bc-attachment>) * 200
    ascii = "x#{tags}"
    multibyte = "é#{tags}"

    plain = Benchmark.realtime { Basecamp::Mentions.mentioned_person_ids(ascii) }
    accented = Benchmark.realtime { Basecamp::Mentions.mentioned_person_ids(multibyte) }

    assert_operator accented, :<, (plain * 10) + 0.5,
      "one non-ASCII character should not change the walk's complexity"
  end

  def test_a_badly_encoded_sgid_is_malformed_rather_than_an_encoding_error
    person = { "id" => 24, "attachable_sgid" => "abc\xC3(def".b }

    assert_raises(Basecamp::UsageError) { Basecamp::Mentions.mention_markup(person) }
  end

  def test_a_float_id_never_mints_a_tag
    # 12.0 == 12 in Ruby; the identity check is on integers.
    sgid = person_sgid(25)

    assert_raises(Basecamp::UsageError) do
      Basecamp::Mentions.mention_markup({ "id" => 25.0, "attachable_sgid" => sgid })
    end
  end

  def test_a_person_that_is_not_a_hash_is_refused_rather_than_raising_no_method_error
    [ [ 26 ], "26", Object.new ].each do |person|
      assert_raises(Basecamp::UsageError) { Basecamp::Mentions.mention_markup(person) }
    end
  end

  def person(id, sgid: nil)
    { "id" => id, "attachable_sgid" => sgid || person_sgid(id) }
  end

  def test_mention_markup_renders_the_write_side_tag
    sgid = person_sgid(10)

    assert_equal %(<bc-attachment sgid="#{sgid}"></bc-attachment>),
                 Basecamp::Mentions.mention_markup(person(10, sgid: sgid))
  end

  def test_mention_markup_refuses_a_person_it_cannot_mention
    assert_raises(Basecamp::UsageError) { Basecamp::Mentions.mention_markup(nil) }
    # No sgid at all — a person projection from a webhook payload, say.
    assert_raises(Basecamp::UsageError) { Basecamp::Mentions.mention_markup({ "id" => 11 }) }
    # An sgid that names somebody else, or a file.
    assert_raises(Basecamp::UsageError) do
      Basecamp::Mentions.mention_markup(person(11, sgid: person_sgid(12)))
    end
    assert_raises(Basecamp::UsageError) do
      Basecamp::Mentions.mention_markup(person(11, sgid: marshal_sgid("gid://bc3/ActiveStorage::Blob/11")))
    end
    # An sgid carrying markup characters, which would break out of the tag.
    assert_raises(Basecamp::UsageError) do
      Basecamp::Mentions.mention_markup({ "id" => 11, "attachable_sgid" => %(x" onload="evil) })
    end
  end

  def test_with_mentions_places_tags_inside_the_first_block
    assert_equal %(<div><bc-attachment sgid="#{person_sgid(13)}"></bc-attachment> On it.</div>),
                 Basecamp::Mentions.with_mentions("<div>On it.</div>", [ person(13) ])
    assert_equal %(<p class="x"><bc-attachment sgid="#{person_sgid(13)}"></bc-attachment> Hi</p>),
                 Basecamp::Mentions.with_mentions(%(<p class="x">Hi</p>), [ person(13) ])
  end

  def test_with_mentions_prefixes_content_that_opens_with_anything_else
    assert_equal %(<bc-attachment sgid="#{person_sgid(14)}"></bc-attachment> plain text),
                 Basecamp::Mentions.with_mentions("plain text", [ person(14) ])
  end

  def test_with_mentions_returns_content_unchanged_with_no_people
    assert_equal "<div>hi</div>", Basecamp::Mentions.with_mentions("<div>hi</div>", [])
  end

  def test_with_mentions_deduplicates_on_the_exact_sgid_only
    already = %(<div><bc-attachment sgid="#{person_sgid(15)}"></bc-attachment> hi</div>)

    # Same person twice in one call, and a person the content already mentions
    # with that exact sgid: neither duplicates the tag.
    assert_equal already, Basecamp::Mentions.with_mentions(already, [ person(15), person(15) ])

    # A DIFFERENT sgid naming the same person is NOT a match. The id an existing
    # tag decodes to is unsigned, so a forged or stale tag naming the right
    # person must not stand in for the real mention.
    stale = %(<div><bc-attachment sgid="#{person_sgid(15, signature: "stale")}"></bc-attachment> hi</div>)
    expanded = Basecamp::Mentions.with_mentions(stale, [ person(15) ])

    assert_equal 2, Basecamp::Mentions.bc_attachment_sgids(expanded).length
  end

  def test_with_mentions_refuses_the_whole_write_when_one_person_is_unusable
    assert_raises(Basecamp::UsageError) do
      Basecamp::Mentions.with_mentions("<div>hi</div>", [ person(16), { "id" => 17 } ])
    end
  end

  def test_mentions_round_trip
    content = Basecamp::Mentions.with_mentions("<div>On it.</div>", [ person(18), person(19) ])

    assert_equal [ 18, 19 ], Basecamp::Mentions.mentioned_person_ids(content)
  end
end
