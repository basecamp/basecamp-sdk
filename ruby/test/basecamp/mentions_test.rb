# frozen_string_literal: true

require "test_helper"

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

  def test_a_line_break_between_the_padding_and_the_separator_names_nobody
    # The padding is right-trimmed off the payload as the reference trims it, so
    # a break sitting between the "=" and the separator leaves the padding in
    # place and the payload is refused. Dropping breaks BEFORE the trim would
    # strip the padding and resolve a person the reference does not. (A break at
    # the very end of the whole sgid is a different case: both trim that with
    # the surrounding whitespace, so both resolve it.)
    unsigned = person_sgid(10).split("--").first
    padding = "=" * ((4 - (unsigned.length % 4)) % 4)

    assert_not_empty padding, "this case needs a payload that pads"
    assert_equal 10, Basecamp::Mentions.person_id_from_sgid("#{unsigned}#{padding}--abc123")
    assert_nil Basecamp::Mentions.person_id_from_sgid("#{unsigned}#{padding}\n--abc123")
    assert_equal 10, Basecamp::Mentions.person_id_from_sgid("#{unsigned}#{padding}--abc123\n")
  end

  def test_a_final_group_with_non_zero_unused_bits_still_decodes
    # The reference decoder is NOT strict about the final group's unused bits,
    # and a stricter one here does not report an error — it makes a real mention
    # silently vanish. Person 10's payload ends in a two-character group whose
    # last character carries four unused bits; setting them decodes to the same
    # bytes and must resolve to the same person.
    sgid = person_sgid(10)
    payload, signature = sgid.split("--", 2)

    assert_equal 2, payload.length % 4, "this case needs a partial final group"
    assert_equal "A", payload[-1]

    assert_equal 10, Basecamp::Mentions.person_id_from_sgid(sgid)
    assert_equal 10, Basecamp::Mentions.person_id_from_sgid("#{payload[0..-2]}B--#{signature}")
  end

  # The character-reference rules below were not read off the reference
  # implementation, they were MEASURED against it: a corpus of ~1000 crafted
  # attribute values was run through both and diffed. Reasoning about what the
  # scanner "should" do is what produced two earlier wrong answers.
  #
  # References are decoded where they occur — in an attribute value — so these
  # go through the walker rather than through person_id_from_sgid.
  def mentions_in_tag(value)
    Basecamp::Mentions.mentioned_person_ids(%(<bc-attachment sgid="#{value}"></bc-attachment>))
  end

  def test_a_decimal_reference_needs_two_digits_or_a_semicolon
    # "&#9B" is not a reference at all in the reference scanner — one digit and
    # no semicolon leaves it literal — while "&#66B" is "B" followed by "B".
    sgid = person_sgid(26)

    assert_equal [ 26 ], mentions_in_tag("&#66;#{sgid[1..]}")
    assert_equal [ 26 ], mentions_in_tag("&#66#{sgid[1..]}")
    assert_empty mentions_in_tag("&#9#{sgid}")
    # Terminated, the same one digit IS a reference — a tab, which the trim erases.
    assert_equal [ 26 ], mentions_in_tag("&#9;#{sgid}")
  end

  def test_a_hex_reference_takes_its_digits_greedily
    # "&#x42B" is U+042B, not "B" followed by "B" — the trailing letter is a hex
    # digit and is consumed.
    sgid = person_sgid(27)

    assert_equal [ 27 ], mentions_in_tag("&#x42;#{sgid[1..]}")
    assert_empty mentions_in_tag("&#x42#{sgid[1..]}")
  end

  def test_a_named_reference_is_matched_against_the_table_not_greedily
    # "&nbspBAh7…" is "&nbsp" followed by text, not a name called "nbspBAh7…".
    # nbsp is one of the references the legacy list accepts unterminated.
    sgid = person_sgid(28)

    assert_equal [ 28 ], mentions_in_tag("&nbsp#{sgid}")
    assert_equal [ 28 ], mentions_in_tag("&nbsp;#{sgid}")
    # Tab is not in the legacy list, so unterminated it stays literal.
    assert_equal [ 28 ], mentions_in_tag("&Tab;#{sgid}")
    assert_empty mentions_in_tag("&Tab#{sgid}")
  end

  def test_whitespace_references_are_erased_at_an_end_and_refused_inside
    # Every whitespace expansion is folded to a space, which is
    # verdict-equivalent: trimmed at either end, refused in the interior.
    sgid = person_sgid(29)

    [ "&nbsp;", "&ensp;", "&ThickSpace;", "&#160;", "&#8194;", "&#x2002;" ].each do |reference|
      assert_equal [ 29 ], mentions_in_tag("#{reference}#{sgid}"),
        "#{reference} should be trimmed at the start"
      assert_empty mentions_in_tag("#{sgid[0, 10]}#{reference}#{sgid[10..]}"),
        "#{reference} should be refused in the interior"
    end
  end

  def test_a_line_break_reference_survives_wherever_it_sits
    # CR and LF are the whitespace a base64 decoder skips in the interior too,
    # so they are NOT folded to a space with the rest.
    sgid = person_sgid(32)

    assert_equal [ 32 ], mentions_in_tag("&NewLine;#{sgid}")
    assert_equal [ 32 ], mentions_in_tag("#{sgid[0, 10]}&NewLine;#{sgid[10..]}")
  end

  def test_a_c1_reference_is_remapped_rather_than_read_as_a_code_point
    # 0x80..0x9F are not code points in HTML, they are Windows-1252 bytes, and
    # the reference implementation remaps them. It decides a verdict here: 0x85
    # is NEL, which IS whitespace and would be trimmed away, while its
    # remapping is an ellipsis, which is not. Reading the number as a code point
    # resolves a person the reference refuses — the accepting direction.
    sgid = person_sgid(33)

    assert_empty mentions_in_tag("&#133;#{sgid}")
    assert_empty mentions_in_tag("&#128;#{sgid}")
    # The hex spelling of the same byte is remapped the same way.
    assert_empty mentions_in_tag("&#x85;#{sgid}")
    # And a code point that really is NEL, reached from above the remapped
    # range, is whitespace and trims — so the assertions above are about the
    # remapping and not about NEL being unrecognized.
    assert_equal [ 33 ], mentions_in_tag("\u0085#{sgid}")
  end

  def test_a_control_reference_cannot_suppress_a_mention
    # The write side deduplicates on the EXACT attachable_sgid, which is what
    # makes this an attack: content whose unescape equalled the authoritative
    # sgid would make the writer skip a mention it must write. A decoder that
    # dropped C0 controls to nothing — as HTML5 says to — would do exactly that.
    sgid = person_sgid(34)
    person = { "id" => 34, "attachable_sgid" => sgid }

    [ "&#1;", "&#8;", "&#0;", "&#x1;" ].each do |reference|
      content = %(<div><bc-attachment sgid="#{sgid}#{reference}"></bc-attachment> hi</div>)
      expanded = Basecamp::Mentions.with_mentions(content, [ person ])

      assert_equal 2, Basecamp::Mentions.bc_attachment_sgids(expanded).length,
        "#{reference} must not suppress the real mention"
    end

    # The control. Without it the assertions above would pass just as happily if
    # the dedupe were broken to never fire at all: here the content really does
    # carry the mention, and exactly one tag must come out.
    already = %(<div><bc-attachment sgid="#{sgid}"></bc-attachment> hi</div>)

    assert_equal 1, Basecamp::Mentions.bc_attachment_sgids(
      Basecamp::Mentions.with_mentions(already, [ person ])
    ).length
  end

  def test_no_character_reference_expands_to_nothing
    # This is the whole suppression question as one property. An attacker can
    # only make unescape(<real sgid> + <suffix>) equal <real sgid> if the suffix
    # decodes to nothing, so a decoder that never yields an empty expansion
    # cannot be turned against the exact-sgid dedupe. The reference table has no
    # empty expansion either, which is why the two agree.
    assert_empty Basecamp::Mentions::NAMED_ENTITIES.values.select(&:empty?)

    sgid = person_sgid(35)
    [ "&#0;", "&#1;", "&#65533;", "&#55296;", "&#1114112;", "&#x0;", "&nosuchref;",
      "&amp;", "&#9;", "&nbsp;" ].each do |reference|
      decoded = Basecamp::Mentions.bc_attachment_sgids(
        %(<bc-attachment sgid="#{sgid}#{reference}"></bc-attachment>)
      ).first

      assert_operator decoded.bytesize, :>, sgid.bytesize,
        "#{reference} decoded to nothing, which would let it suppress a mention"
    end
  end

  def test_a_reference_expanding_to_base64_characters_decides_a_verdict
    # "&fjlig;" is the one two-character expansion in the reference table that
    # is entirely base64, so it can complete a payload on its own rather than
    # merely spoiling one. An insertion that both implementations refuse would
    # prove nothing here; this one has to RESOLVE.
    sgid = person_sgid(36)
    position = sgid.index("fj")

    if position
      assert_equal [ 36 ], mentions_in_tag("#{sgid[0, position]}&fjlig;#{sgid[position + 2..]}")
    else
      assert_equal "fj", Basecamp::Mentions::NAMED_ENTITIES["fjlig"]
    end
  end

  def test_mixing_encodings_in_one_value_is_refused_rather_than_raised
    # Every expansion reaches the byte string as bytes. A numeric reference
    # beside a named one whose expansion is non-ASCII used to build one gsub
    # result out of two encodings and raise Encoding::CompatibilityError out of
    # a public method — on every summarize, over whatever BC3 served.
    assert_empty Basecamp::Mentions.mentioned_person_ids(%q(<bc-attachment sgid="&#233;&nbsp;abc">))
    assert_empty Basecamp::Mentions.mentioned_person_ids(%(<bc-attachment sgid="&ensp;é">))
    assert_empty Basecamp::Mentions.mentioned_person_ids(%(<bc-attachment sgid="&nbsp;#{"\xC3(".b}">))
  end

  def test_an_extracted_sgid_is_always_bytes
    # The dedupe set in with_mentions is keyed on this, and a UTF-8 sgid and its
    # byte-identical binary twin are neither eql? nor hash-equal.
    [ %q(<bc-attachment sgid="abc">), %q(<bc-attachment sgid="&nbsp;abc">),
      %q(<bc-attachment sgid="&#233;abc">) ].each do |document|
      sgid = Basecamp::Mentions.bc_attachment_sgids(document).first

      assert_equal Encoding::BINARY, sgid.encoding, document
    end
  end

  def test_a_non_ascii_sgid_is_not_mentioned_twice
    # The person read hands back text and the walker hands back bytes. Keyed on
    # anything but bytes, the same sgid splits the set and the mention is
    # written a second time — a posted comment mentioning someone twice.
    sgid = "\u00A0#{person_sgid(37)}"
    person = { "id" => 37, "attachable_sgid" => sgid }
    content = %(<p><bc-attachment sgid="#{sgid}"></bc-attachment> hi</p>)

    assert_equal 1, Basecamp::Mentions.bc_attachment_sgids(
      Basecamp::Mentions.with_mentions(content, [ person ])
    ).length
  end

  def test_one_stray_byte_does_not_disarm_the_whitespace_trim
    # The reference decodes a character at EACH END independently of the rest of
    # the value. Choosing an alphabet from whether the whole string is valid
    # UTF-8 loses a mention whenever a stray byte sits anywhere — including in
    # the signature half the separator throws away, which is the reachable case:
    # a clean payload, a non-ASCII space in front of it, and one bad byte in the
    # digest.
    sgid = person_sgid(38)

    [ "\u00A0", "\u2003", "\u3000", "\u0085", "\t" ].each do |space|
      assert_equal 38, Basecamp::Mentions.person_id_from_sgid("#{space}#{sgid}\xFF".b),
        "#{space.inspect} before a payload whose digest carries a stray byte"
      assert_equal 38, Basecamp::Mentions.person_id_from_sgid("#{space}#{sgid}\xFF#{space}".b),
        "#{space.inspect} at both ends, stray byte still in the digest"
      # A stray byte in the PAYLOAD half is refused by both, whatever the
      # whitespace does — so the assertions above are about the trim and not
      # about strays being tolerated.
      assert_nil Basecamp::Mentions.person_id_from_sgid("#{space}\xFF#{sgid}".b)
    end
  end

  def test_characters_that_look_like_spaces_but_are_not_are_kept
    # The control for the trim tests above. Without it they would pass just as
    # happily if the trim removed everything it did not recognize: these three
    # are NOT in the reference's space set, so an sgid they prefix stays
    # undecodable.
    sgid = person_sgid(40)

    { "zero-width space" => "\u200B", "NUL" => "\u0000",
      "Mongolian vowel separator" => "\u180E" }.each do |name, character|
      assert_nil Basecamp::Mentions.person_id_from_sgid("#{character}#{sgid}".b), name
    end
  end

  def test_a_truncated_character_ends_the_trim_rather_than_extending_it
    # A byte that starts no character, or starts one that is cut short, is not
    # whitespace and stops the run — the reference's rune decode does the same.
    sgid = person_sgid(39)

    assert_nil Basecamp::Mentions.person_id_from_sgid("\xC2#{sgid}".b)
    assert_nil Basecamp::Mentions.person_id_from_sgid("\xE2\x80#{sgid}".b)
    # The whole of that character, though, is whitespace and is trimmed.
    assert_equal 39, Basecamp::Mentions.person_id_from_sgid("\xE2\x80\x83#{sgid}".b)
  end

  def test_invalid_utf8_is_undecodable_rather_than_an_encoding_error
    broken = "abc\xC3(def".b.force_encoding(Encoding::UTF_8)

    assert_nil Basecamp::Mentions.person_id_from_sgid(broken)
    assert_raises(Basecamp::UsageError) do
      Basecamp::Mentions.mention_markup({ "id" => 7, "attachable_sgid" => broken })
    end
  end

  def test_a_reference_outside_the_table_can_only_lose_a_mention_never_add_one
    # The bound is one-directional by construction: a reference left literal
    # contributes "&" and ";", which no base64 alphabet accepts.
    sgid = person_sgid(31)

    assert_empty mentions_in_tag("&nosuchref;#{sgid}")
    assert_empty mentions_in_tag("#{sgid[0, 10]}&nosuchref;#{sgid[10..]}")
  end

  def test_a_person_id_past_the_64_bit_range_is_refused
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/Person/99999999999999999999999"))
  end

  def test_an_escape_naming_an_ascii_byte_in_the_authority_is_refused
    # Measured against the reference by planting every printable byte mid-host:
    # it refuses an escape that names an ASCII byte, excepting "%25" which names
    # the percent itself, and accepts one above ASCII. Without this the port
    # resolved 95 hosts the reference refuses — the accepting direction, and the
    # write side's only authenticity-adjacent check is whether an sgid names the
    # person it is given.
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc%203/Person/12"))
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc%2F3/Person/12"))
    assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc%7e3/Person/12"))
    # The two exceptions.
    assert_equal 12, Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc%253/Person/12"))
    assert_equal 12, Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://b%C3%A9c3/Person/12"))
    # And an escape in the PATH is still decoded, which is a different rule.
    assert_equal 12, Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://bc3/Pe%72son/12"))
  end

  def test_an_authority_that_is_only_a_port_still_names_a_host
    # The reference's non-empty test is on a field that carries the PORT, with
    # userinfo split off at the last "@". So an authority of just ":8080" IS a
    # host there and resolves, while "user@" is not — and Ruby's URI reports an
    # empty host for both, which refused eight shapes the reference accepts.
    [ ":8080", ":80", ":0", ":", ":65535", "user@:8080", "user:pw@:8080" ].each do |authority|
      assert_equal 12, Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://#{authority}/Person/12")),
        "authority #{authority.inspect} should name a host"
    end

    # The control: an authority that is ONLY userinfo has no host, in either.
    [ "user@", "@", "" ].each do |authority|
      assert_nil Basecamp::Mentions.person_id_from_sgid(marshal_sgid("gid://#{authority}/Person/12")),
        "authority #{authority.inspect} should name no host"
    end
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

    plain = elapsed { Basecamp::Mentions.mentioned_person_ids(ascii) }
    accented = elapsed { Basecamp::Mentions.mentioned_person_ids(multibyte) }

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

  # Monotonic, and no require: `benchmark` left the default gems in Ruby 4.0.
  def elapsed
    started = Process.clock_gettime(Process::CLOCK_MONOTONIC)
    yield
    Process.clock_gettime(Process::CLOCK_MONOTONIC) - started
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
