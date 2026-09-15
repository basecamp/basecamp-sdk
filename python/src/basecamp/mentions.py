"""Mention helpers over Basecamp rich text.

A mention in Basecamp rich text is a ``<bc-attachment>`` whose ``sgid``
attribute is the mentioned person's ``attachable_sgid``
(``doc/api/sections/rich_text.md``, "Inserting a mention"). BC3 renders the same
tag back with ``content-type="application/vnd.basecamp.mention"`` and an avatar
figure inside it, but the sgid is the only part of the markup that names the
person on both the write and the read side, so both helpers here work from it:

* :func:`mentioned_person_ids` reads the person ids a rich text names, by
  decoding the sgid of every ``<bc-attachment>`` and keeping the ones that point
  at a Person.
* :func:`mention_markup` writes the tag for a person, from their
  ``attachable_sgid``.

An ``attachable_sgid`` is a Rails SignedGlobalID: a base64 payload, then ``--``,
then an HMAC only BC3 can verify. The payload is an envelope carrying the global
id -- ``gid://bc3/Person/1049715915`` -- as a string, and that string is what
these helpers read. They do not (and cannot) verify the signature; what they
decode is the same person id BC3 renders into the mention's avatar, read off
content the API already served, and a caller that needs the id verified reads
the person back through ``people.get``.

That sets a trust boundary between the two sides. READING --
:func:`mentioned_person_ids`, :func:`person_id_from_sgid` -- describes what a
text says it mentions, and unsigned is fine for description: the ids are
reported, not acted on as proof. WRITING -- :func:`with_mentions`,
``CommentsService.expand_mentions`` -- never treats an unsigned id as proof that
a valid mention already exists: a forged or stale sgid in caller-supplied
content naming the right id would otherwise make the writer skip the
authoritative people read and post a tag Basecamp will not honour, so the person
is silently not mentioned. ``CommentsService.expand_mentions`` therefore
resolves every requested person through ``people.get`` and deduplicates only
against the exact ``attachable_sgid`` string that read returned. The pure
helpers beneath it -- :func:`with_mentions`, :func:`mention_markup` -- take
person payloads the caller supplied and can only check that an sgid is
well-formed and names the person it is given, never that it is authentic: hand
them people the API returned, not people assembled from content. Do not reuse
the read-side helpers to decide whether a write can be skipped.

The markup is read as BC3 serves it: a sanitized tree of the tags
``doc/api/sections/rich_text.md`` allows, which has no raw-text elements. The
tag walk skips comments and quoted attribute values but does not model
``<script>`` or ``<style>`` content, which BC3 strips on write; a caller reading
mentions out of content it authored itself should not put a ``bc-attachment``
inside such an element and expect it ignored.

The envelope is decoded structurally, never searched as bytes, so a Person gid
that merely appears inside some other value -- a Document gid built from one, a
purpose string that looks like one -- is not a mention, and the envelope's
purpose must be ``attachable``, the one BC3 accepts in rich text. Three
envelopes are read: Rails' current Marshal layout
``{"_rails" => {"data" => gid, "pur" => purpose}}``, the older Marshal layout
``{"gid" => gid, "purpose" => ..., "expires_at" => ...}``, and the JSON spelling
of either, which Rails' JSON message serializer emits.
"""

from __future__ import annotations

import base64
import binascii
import ipaddress
import json
import string
from collections.abc import Iterable, Mapping
from html.entities import html5
from typing import Any
from urllib.parse import unquote

from basecamp.errors import UsageError

__all__ = [
    "mention_markup",
    "mentioned_person_ids",
    "person_id_from_sgid",
    "with_mentions",
]

#: The SignedGlobalID purpose BC3 mints attachable sgids with
#: (``doc/api/sections/rich_text.md``: ``attachable_sgid``). Pinned by the
#: purpose cases in ``tests/test_mentions.py``, so a rename upstream breaks a
#: test here rather than silently turning every mention invisible.
_SGID_PURPOSE_ATTACHABLE = "attachable"

#: The largest id a Person gid may carry. BC3's ids are 64-bit signed, and
#: Go's decoder refuses anything wider; Python's int would not.
_MAX_PERSON_ID = 2**63 - 1

#: Bounds the decoded sgid payload. A Person sgid's payload is under 200 bytes;
#: the cap keeps a hostile one from costing more than its own size to reject.
_MAX_SGID_PAYLOAD_BYTES = 4096
#: The same bound on the base64 form (4/3 of the payload, plus padding), checked
#: before anything is allocated.
_MAX_SGID_ENCODED_BYTES = _MAX_SGID_PAYLOAD_BYTES // 3 * 4 + 4

#: Bounds nesting in a Marshal payload; an envelope is two deep.
_RUBY_MARSHAL_MAX_DEPTH = 32

_SPACE = " \t\n\r\f"

#: Python's ``str.strip()`` also removes these four C0 separators; Go's
#: ``unicode.IsSpace`` does not treat them as space. Trimming one off an sgid
#: that Go would have kept turns an undecodable value into a decodable one --
#: and the write side's only gate is whether the sgid names the person.
_NOT_GO_SPACE = "\x1c\x1d\x1e\x1f"


def go_trim_space(value: str) -> str:
    """``strings.TrimSpace``: Python's ``strip()`` minus the four C0 separators.

    Shared with the recording router, which trims its routing keys the same way
    Go does.
    """
    start, end = 0, len(value)
    while start < end and value[start].isspace() and value[start] not in _NOT_GO_SPACE:
        start += 1
    while end > start and value[end - 1].isspace() and value[end - 1] not in _NOT_GO_SPACE:
        end -= 1
    return value[start:end]


#: What Go's ``url.Parse`` leaves unescaped in a host: the unreserved
#: characters, the sub-delims, and the few it admits because a host cannot
#: percent-encode an ASCII byte. Go checks this set for ASCII bytes ONLY --
#: anything at or above 0x80 passes untested, which is why a non-ASCII host is
#: legal there.
_HOST_ALLOWED_PUNCTUATION = "-._~!$&'()*+,;=:[]<>\""


def _split_gid(gid: str) -> tuple[str, str] | None:
    """A gid's authority and path, split as Go's ``url.Parse`` splits them.

    Hand-written rather than handed to ``urlparse``, because Python's parser
    implements a DIFFERENT standard -- WHATWG rather than RFC 3986 as Go reads
    it -- and it disagrees in BOTH directions. Stricter in places (an unmatched
    "]" is a ValueError where Go keeps the host), but also LAXER, which is the
    dangerous one: ``urlsplit`` silently strips a tab, CR or LF, so
    "gid://bc3/Person/104\n9715915" repairs itself into person 1049715915,
    where Go refuses the URL outright and names nobody. A read that invents a
    mention the API never made is worse than one that declines to build a tag.

    The two above are examples, not an inventory -- three were found at
    different times, and an enumeration in a comment is exactly the thing that
    stops the next sweep happening. The shape is what matters: a general parser
    normalises and rejects on its own schedule, a gid is a fixed trivial form,
    and parity is established by differential against a linked ``url.Parse``
    rather than by reasoning about either. The harness lives with the port's
    review notes.
    """
    if gid[:6].casefold() != "gid://":
        return None
    rest = gid[6:]
    cut = len(rest)
    for index, character in enumerate(rest):
        if character in "/?#":
            cut = index
            break
    path = rest[cut:]
    for terminator in "?#":
        position = path.find(terminator)
        if position >= 0:
            path = path[:position]
    return rest[:cut], path


def _valid_authority(authority: str) -> bool:
    """Whether Go's ``url.Parse`` would accept this authority's host.

    Python's parser does not look at it: ``gid://bc 3/Person/1`` and
    ``gid://bc3:xx/Person/1`` both parse cleanly here and fail there, so each
    named a person in this SDK and nobody in Go -- on the write side, where the
    only gate is whether the sgid names the person.

    Checked against a linked ``url.Parse`` rather than described from it,
    because the obvious reading of "validate the authority" is wrong in three
    separate ways: Go strips USERINFO before validating (``user@bc3`` is host
    ``bc3``), it permits any non-ASCII byte in a host, and it REFUSES a
    percent-escape of an ASCII byte (``%41``) that a permissive reading waves
    through.
    """
    if not authority:
        return False
    # Userinfo is not part of the host -- Go splits on the LAST "@" -- but it is
    # still validated, and against a NARROWER set than the host: no non-ASCII,
    # no brackets, no space.
    userinfo, at, host = authority.rpartition("@")
    if at and not _valid_userinfo(userinfo):
        return False
    if not host:
        return False
    # A "[" anywhere commits the host to the bracketed form: Go accepts "]" as
    # an ordinary host character but never a stray "[".
    if "[" in host:
        return _valid_bracketed_host(host)
    # An optional port begins at the last ":" and must be digits or empty.
    _, colon, port = host.rpartition(":")
    if colon and port and not (port.isascii() and port.isdigit()):
        return False
    return _valid_host_characters(host)


def _valid_bracketed_host(host: str) -> bool:
    # A "[" may appear only as the opening bracket; a "]" inside is ordinary,
    # because the CLOSING one is the last.
    if not host.startswith("[") or "[" in host[1:]:
        return False
    closing = host.rfind("]")
    if closing < 0:
        return False
    port = host[closing + 1 :]
    if port and not (port.startswith(":") and (port == ":" or (port[1:].isascii() and port[1:].isdigit()))):
        return False
    inside = host[1:closing]
    address, zoned, zone = inside.partition("%25")
    # A "%" in the ADDRESS half is refused outright. Go unescapes that half in
    # host mode, where an escape may carry only a byte at or above 0x80, and
    # then hands the result to an IPv6 parser that no such byte survives — so
    # every spelling fails there. Python's `IPv6Address` accepts a `%scope`
    # suffix of its own, which would otherwise let the whole family through:
    # "[fe80::1%ab%25eth0]" parsed as address "fe80::1%ab" and named a person.
    # This also settles a bare "%": it is not a zone marker, so partition leaves
    # it in the address half, and Go refuses "[fe80::1%eth0]" too.
    if "%" in address:
        return False
    # RFC 6874 spells a zone "%25<zone>". The zone may not be empty, and it is
    # held to its own character rule -- see _valid_host_characters.
    if zoned and (not zone or not _valid_host_characters(zone, zone_identifier=True)):
        return False
    try:
        ipaddress.IPv6Address(address)
    except ValueError:
        # IPvFuture ("[v1.fe80::a+en1]") and a bare IPv4 reach here, and Go
        # refuses both.
        return False
    return True


#: What Go's ``validUserinfo`` permits, which is not what it permits in a host.
_USERINFO_ALLOWED_PUNCTUATION = "-._:~!$&'()*+,;=%@"


def _valid_userinfo(userinfo: str) -> bool:
    index = 0
    while index < len(userinfo):
        character = userinfo[index]
        if character == "%":
            # Go unescapes the userinfo as well as validating its character
            # set, so a malformed escape fails the whole parse. Unlike a host,
            # an escape of an ASCII byte is allowed here.
            escape = userinfo[index + 1 : index + 3]
            if len(escape) != 2 or not all(c in string.hexdigits for c in escape):
                return False
            index += 3
            continue
        if not (character.isascii() and (character.isalnum() or character in _USERINFO_ALLOWED_PUNCTUATION)):
            return False
        index += 1
    return True


def _valid_escapes(text: str) -> bool:
    """Whether every percent-escape in ``text`` is well formed.

    Go unescapes a fragment when it sets one, so a malformed escape there fails
    the whole parse even though nothing else about a fragment is checked.
    """
    index = text.find("%")
    while index >= 0:
        escape = text[index + 1 : index + 3]
        if len(escape) != 2 or not all(c in string.hexdigits for c in escape):
            return False
        index = text.find("%", index + 3)
    return True


def _must_escape_in_host(byte: int) -> bool:
    """Go's ``shouldEscape(c, encodeHost)``: everything outside the allowed set.

    A byte at or above 0x80 must be escaped by this rule — which is what makes
    a percent-escape of a non-ASCII byte legal in a host and an escape of an
    ASCII one illegal.
    """
    character = chr(byte)
    return not (character.isascii() and (character.isalnum() or character in _HOST_ALLOWED_PUNCTUATION))


def _valid_host_characters(text: str, *, zone_identifier: bool = False) -> bool:
    """Go's ``unescape(s, encodeHost)`` — or ``encodeZone`` for a zone.

    The two differ only in what an escape may carry. A host may escape a
    non-ASCII byte and nothing else; a ZONE may escape anything it could have
    written literally, plus a space, because Windows puts spaces in zone
    identifiers. So ``[fe80::1%25%20en0]`` is a host Go accepts and
    ``[fe80::1%25en 0]`` — the same byte, written literally — is not.
    """
    index = 0
    while index < len(text):
        character = text[index]
        if character == "%":
            escape = text[index + 1 : index + 3]
            if len(escape) != 2 or not all(c in string.hexdigits for c in escape):
                return False
            if text[index : index + 3] != "%25":
                byte = int(escape, 16)
                if zone_identifier:
                    if byte != 0x20 and _must_escape_in_host(byte):
                        return False
                elif byte < 0x80:
                    return False
            index += 3
            continue
        if character.isascii() and _must_escape_in_host(ord(character)):
            return False
        index += 1
    return True


def mentioned_person_ids(rich_text: str) -> list[int]:
    """The ids of the people a rich text mentions.

    The Person named by the sgid of each ``<bc-attachment>``, in document order,
    with repeats removed. Attachments that are not mentions -- files, images,
    embeds -- are skipped, as is any sgid that does not decode to a Person.

    This is the read side: a description of what the text says, from sgids whose
    signatures cannot be checked here. Report it; do not treat an id in it as
    proof that a valid mention exists (see the trust boundary in the module
    docstring).

    Every ``<bc-attachment>`` in the text counts, including one inside a
    ``<blockquote>``: BC3 notifies quoted mentions too, so the read matches what
    the server does with the write.
    """
    ids: list[int] = []
    seen: set[int] = set()
    for sgid in _bc_attachment_sgids(rich_text):
        person_id = person_id_from_sgid(sgid)
        if person_id is None or person_id in seen:
            continue
        seen.add(person_id)
        ids.append(person_id)
    return ids


def person_id_from_sgid(sgid: str) -> int | None:
    """The Person id an ``attachable_sgid`` names, or ``None``.

    ``None`` when the sgid does not decode, or names something other than a
    Person (a file attachment's sgid names an ``ActiveStorage::Blob``).

    This reads the id out of the sgid's payload; it does not verify the sgid's
    signature, which only BC3 can. It is a read-side helper: never use its
    answer to decide that a write may skip the authoritative people read (see
    the trust boundary in the module docstring).
    """
    gid = _global_id_from_sgid(sgid)
    if gid is None:
        return None
    # Go cuts the fragment off FIRST and only then refuses control characters,
    # so a control character behind "#" is not the URL's problem — while a
    # malformed escape in the fragment still fails the parse.
    located, hashed, fragment = gid.partition("#")
    if hashed and not _valid_escapes(fragment):
        return None
    # Refused BEFORE parsing, because Python's URL parser would not refuse it:
    # `urlsplit` STRIPS tab, CR and LF from the input (the WHATWG rule), so
    # "gid://bc3/Person/104\n9715915" would parse as a clean id and this helper
    # would report a mention of somebody the API never named. Go's net/url
    # rejects any ASCII control character outright, and so does this.
    if any(character < " " or character == "\x7f" for character in located):
        return None
    parsed = _split_gid(located)
    if parsed is None:
        return None
    authority, path = parsed
    if not _valid_authority(authority):
        return None
    # A GlobalID path is exactly "/<Model>/<id>": no more, no less, and read
    # DECODED -- "gid://bc3/Pers%6fn/123" names a Person, as Go's url.Path does.
    model, separator, raw_id = unquote(path).removeprefix("/").partition("/")
    if separator != "/" or model != "Person" or not raw_id:
        return None
    # `str.isdigit` is true for non-ASCII digits, which `int()` would then
    # happily parse into an id BC3 never wrote. It is also what refuses a
    # percent-encoded control character, now that the path is decoded.
    if not (raw_id.isascii() and raw_id.isdigit()):
        return None
    person_id = int(raw_id)
    # Python's int is arbitrary-precision where BC3's id is not: Go's
    # ParseInt(..., 64) refuses anything past int64 rather than reporting a
    # mention no other SDK in this repo could even produce.
    return person_id if 0 < person_id <= _MAX_PERSON_ID else None


def mention_markup(person: Mapping[str, Any]) -> str:
    """The ``<bc-attachment>`` that mentions a person, from their ``attachable_sgid``.

    The write-side form in ``doc/api/sections/rich_text.md``, which BC3 expands
    into the avatar figure on read. Raises :class:`~basecamp.errors.UsageError`
    when the person carries no ``attachable_sgid`` -- the case for a person
    projection that came from somewhere other than a people read (a webhook
    payload, say) -- and when the sgid does not name the person it is given.
    That is all it can check: it cannot verify the signature, so the person must
    come from the API -- a ``people.get``, a recording's ``creator`` or
    ``assignees`` -- not be assembled from an sgid found in content.
    """
    if person is None:
        raise UsageError("cannot mention a missing person")
    # A body that is not an object reaches here as a list, a string or a number.
    # Go's typed decode refuses those before `MentionMarkup` ever sees one, and
    # it must stay a refusal rather than an `AttributeError` off `person.get`:
    # this is the write path, so the only safe outcome is that nothing posts.
    if not isinstance(person, Mapping):
        raise UsageError(f"cannot mention a person read as {type(person).__name__}")
    # Go's order, so the diagnosis matches: absent sgid, then a malformed one,
    # then one that names somebody else. Go reads a missing id as 0 and lets
    # the last check report it; Python's dict can omit the key entirely, so the
    # id is normalised here rather than checked in a rung of its own.
    person_id = person.get("id")
    if not isinstance(person_id, int) or isinstance(person_id, bool):
        # Go reads an absent id as the zero value and lets the sgid/id check
        # report it; a real id it names verbatim, negative or not.
        person_id = 0
    sgid = person.get("attachable_sgid") or ""
    if not isinstance(sgid, str) or not sgid:
        raise UsageError(
            f"person {person_id} has no attachable_sgid to mention",
            hint="read the person through people.get to obtain one",
        )
    if any(character in sgid for character in "\"'<>&"):
        raise UsageError(f"person {person_id} has a malformed attachable_sgid")
    # The tag mentions whoever the sgid names. Refuse to write one that names
    # someone else -- or a file -- under this person's id.
    # No `person_id <= 0` guard: `person_id_from_sgid` never returns one, so a
    # non-positive id simply fails this comparison — and Go reports the id it
    # was given rather than a normalised one.
    if person_id_from_sgid(sgid) != person_id:
        raise UsageError(
            f"person {person_id}'s attachable_sgid does not name that person",
            hint="read the person through people.get to obtain their own",
        )
    return f'<bc-attachment sgid="{sgid}"></bc-attachment>'


def with_mentions(content: str, people: Iterable[Mapping[str, Any]]) -> str:
    """Content that mentions each of the given people.

    For posting as a comment or a Campfire line. A person whose exact
    ``attachable_sgid`` the content already carries is left alone, so passing the
    same person twice -- or a person the author already mentioned with that sgid
    -- never duplicates the mention; the rest are added at the start of the
    content, inside its first ``<p>`` or ``<div>`` when it opens with one, so
    they render on the first line rather than as a block of their own.

    This is the write side, and it deduplicates on the sgid string alone, never
    on the person id an existing tag's sgid decodes to: that id is unsigned, and
    a forged or stale tag naming the right person must not stand in for the real
    mention (see the trust boundary in the module docstring). Every person needs
    their own ``attachable_sgid``, and it must be one the API returned: this
    helper can check that an sgid is well-formed and names the person, not that
    it is authentic (see :func:`mention_markup`). The account-bound
    ``CommentsService.expand_mentions`` resolves ids to people first and is the
    entry point that carries that guarantee.
    """
    present = set(_bc_attachment_sgids(content))
    tags: list[str] = []
    for person in people:
        # Rendered before the dedupe check, so a person the content already
        # mentions is still validated rather than waved through.
        tag = mention_markup(person)
        sgid = person["attachable_sgid"]
        if sgid in present:
            continue
        present.add(sgid)
        tags.append(tag)
    if not tags:
        return content
    prefix = " ".join(tags) + " "
    end = _leading_block_end(content)
    if end >= 0:
        return content[:end] + prefix + content[end:]
    return prefix + content


# --- Markup walking ---------------------------------------------------------


def _is_space(character: str) -> bool:
    return character in _SPACE


def _is_tag_name_char(character: str) -> bool:
    """What may appear in a tag name.

    The whole name is consumed, punctuation included, so ``<bc-attachment:preview``
    or ``<bc-attachment_x`` is its own name and never compares equal to
    ``bc-attachment``.
    """
    return not _is_space(character) and character not in "/><=\"'"


def _is_tag_name_end(character: str) -> bool:
    return _is_space(character) or character in "/>"


def _bc_attachment_sgids(text: str) -> list[str]:
    """The ``sgid`` attribute of every ``<bc-attachment>``, in document order.

    Walks the markup as a stream of tags rather than pattern-matching for one
    tag name, so a ``<bc-attachment>`` inside an HTML comment or inside another
    element's quoted attribute is not an element; and tokenizes each tag's
    attributes rather than pattern-matching them, so a ``>`` inside a quoted
    value does not end the tag, an ``sgid=`` inside another attribute's value is
    not an attribute, either quote style works, attribute order and case are
    free, the first ``sgid`` attribute wins as in HTML, and entity escapes in the
    value are decoded as a browser would.
    """
    sgids: list[str] = []
    length = len(text)
    pos = 0
    while pos < length:
        opening = text.find("<", pos)
        if opening < 0:
            break
        pos = opening + 1
        if text.startswith("!--", pos):
            stop = text.find("-->", pos)
            if stop < 0:
                return sgids  # an unterminated comment swallows the rest
            pos = stop + 3
            continue
        if pos < length and text[pos] in "!?/":
            stop = text.find(">", pos)
            if stop < 0:
                return sgids
            pos = stop + 1
            continue
        name_end = pos
        while name_end < length and _is_tag_name_char(text[name_end]):
            name_end += 1
        if name_end == pos:
            continue  # a bare "<" in text
        sgid, end, closed = _parse_attributes(text, name_end)
        if not closed:
            return sgids  # an unterminated tag: nothing after it is markup
        if sgid and text[pos:name_end].casefold() == "bc-attachment":
            sgids.append(sgid)
        pos = end
    return sgids


def _parse_attributes(text: str, pos: int) -> tuple[str, int, bool]:
    """Walk one opening tag's attributes from just after its name.

    Returns the decoded value of the first ``sgid`` attribute (present-but-empty
    included, as HTML resolves a repeated attribute), the index after the
    closing ``>``, and whether the tag was closed at all.
    """
    length = len(text)
    sgid = ""
    sgid_seen = False
    while pos < length:
        while pos < length and (_is_space(text[pos]) or text[pos] == "/"):
            pos += 1
        if pos >= length:
            return sgid, pos, False
        if text[pos] == ">":
            return sgid, pos + 1, True
        name_start = pos
        while pos < length and not _is_space(text[pos]) and text[pos] not in "=>/":
            pos += 1
        name = text[name_start:pos]
        while pos < length and _is_space(text[pos]):
            pos += 1
        value = ""
        if pos < length and text[pos] == "=":
            pos += 1
            while pos < length and _is_space(text[pos]):
                pos += 1
            if pos < length and text[pos] in "\"'":
                quote = text[pos]
                pos += 1
                closing = text.find(quote, pos)
                if closing < 0:
                    return sgid, length, False
                value = text[pos:closing]
                pos = closing + 1
            else:
                value_start = pos
                while pos < length and not _is_space(text[pos]) and text[pos] != ">":
                    pos += 1
                value = text[value_start:pos]
        if not name:
            # A stray "=" or quote where a name should be: step over it.
            pos += 1
            continue
        if not sgid_seen and name.casefold() == "sgid":
            sgid_seen = True
            sgid = _unescape_like_go(value)
    return sgid, pos, False


def _leading_block_end(content: str) -> int:
    """The index just past the opening ``<p ...>`` or ``<div ...>`` a rich text starts with.

    ``-1`` when it starts with anything else, so mentions can be placed inside
    the first block rather than as a bare prefix in front of it. The tag's
    attributes are scanned quote-aware: a ``>`` inside an attribute value does
    not end it.
    """
    length = len(content)
    start = 0
    while start < length and _is_space(content[start]):
        start += 1
    for name in ("<p", "<div"):
        if length < start + len(name) or content[start : start + len(name)].casefold() != name:
            continue
        after = start + len(name)
        if after < length and not _is_tag_name_end(content[after]):
            continue
        _, end, closed = _parse_attributes(content, after)
        return end if closed else -1
    return -1


# --- Entity decoding -------------------------------------------------------
#
# Go's `html.UnescapeString` and Python's `html.unescape` implement DIFFERENT
# specifications, and the difference decides whether a mention is seen. Python
# follows HTML5, which drops a numeric reference naming a C0 control or DEL to
# the empty string; Go emits the character. That direction is the dangerous
# one: an attacker-supplied `sgid="<real sgid>&#1;"` unescapes to the real sgid
# under HTML5, matches what the people read returned, and makes the WRITER skip
# the mention — the forged-tag suppression the whole design exists to prevent.
# Go keeps the control character, the strings differ, and the mention is
# written.
#
# So numeric references are decoded here, to Go's rules, read off a linked
# `html.UnescapeString` oracle rather than off its source:
#
#   - decimal without a terminating ";" needs TWO digits ("&#9" is literal,
#     "&#09" is a tab); hex without one needs only ONE ("&#x9" is a tab)
#   - no digits at all decodes only as "&#x;" / "&#X;", to U+FFFD
#   - 0, the surrogates and anything past U+10FFFF are U+FFFD
#   - 0x80-0x9F map through the C1 table below
#
# NAMED references are Python's, because they were measured to agree across
# every name in the table with and without a semicolon, including the
# longest-match-against-the-table rule that a greedy name scan would get wrong
# ("&nbspBAh7" is a non-breaking space followed by "BAh7" in both).

#: The HTML5 C1 replacement table, as Go's `html` package applies it. Extracted
#: from the oracle and pinned by a test, so a hand-transcription error cannot
#: quietly change which character an sgid carries.
_C1_REPLACEMENTS = (
    "\u20ac\u0081\u201a\u0192\u201e\u2026\u2020\u2021"
    "\u02c6\u2030\u0160\u2039\u0152\u008d\u017d\u008f"
    "\u0090\u2018\u2019\u201c\u201d\u2022\u2013\u2014"
    "\u02dc\u2122\u0161\u203a\u0153\u009d\u017e\u0178"
)

#: Names Python's table decodes and Go's does not.
#:
#: Measured rather than assumed, and the basis is stated so it can be re-taken
#: rather than believed: every name in the table, in five syntactic forms each,
#: through a linked ``html.UnescapeString``. That is exhaustive over the table,
#: which is finite -- it is NOT a claim about inputs outside it. An earlier
#: version of this set was built from the first 400 names and was wrong.
#:
#: Their expansions are neither whitespace nor base64 characters, so they
#: cannot change which person an sgid names either way -- but "cannot matter"
#: is the argument this port has watched collapse twice, so they are excluded
#: rather than reasoned about.
_NOT_IN_GO_TABLE = frozenset({"nGt;", "nLt;"})

_MAX_ENTITY_NAME = max(len(name) for name in html5)


def _unescape_like_go(value: str) -> str:
    """Decode entity references the way Go's ``html.UnescapeString`` does."""
    if "&" not in value:
        return value
    out: list[str] = []
    index = 0
    while index < len(value):
        character = value[index]
        if character != "&":
            out.append(character)
            index += 1
            continue
        consumed, text = _reference_at(value, index)
        if consumed:
            out.append(text)
            index += consumed
        else:
            out.append("&")
            index += 1
    return "".join(out)


def _reference_at(value: str, start: int) -> tuple[int, str]:
    """One entity reference at ``start``: characters consumed, and what it means.

    ``(0, "")`` when there is no reference there and the ``&`` is literal.
    """
    if value.startswith("&#", start):
        return _numeric_reference_at(value, start)
    # Longest match against the TABLE, not the longest run of name characters:
    # "&nbspBAh7" is "&nbsp" followed by "BAh7", which a greedy name scan reads
    # as one unknown name and leaves undecoded.
    #
    # Bounded by the characters actually present, though, and that bound is
    # load-bearing rather than tidy. A table name is [A-Za-z0-9]+ with an
    # optional ";", so nothing longer than the run following "&" can match, and
    # sweeping every length to the longest name in the table regardless would
    # cost a few hundred comparisons per ampersand. That is linear in the input
    # with a constant big enough to matter on content an author controls, and a
    # run of ampersands is its worst case: here each one costs a single look.
    limit = start + 1
    ceiling = min(len(value), limit + _MAX_ENTITY_NAME)
    while limit < ceiling and value[limit].isascii() and value[limit].isalnum():
        limit += 1
    length = limit - (start + 1)
    if limit < len(value) and value[limit] == ";":
        length = min(length + 1, _MAX_ENTITY_NAME)
    while length > 0:
        name = value[start + 1 : start + 1 + length]
        if name not in _NOT_IN_GO_TABLE:
            replacement = html5.get(name)
            if replacement is not None:
                return length + 1, replacement
        length -= 1
    return 0, ""


def _numeric_reference_at(value: str, start: int) -> tuple[int, str]:
    cursor = start + 2
    hex_form = cursor < len(value) and value[cursor] in "xX"
    if hex_form:
        cursor += 1
    digits_start = cursor
    allowed = string.hexdigits if hex_form else string.digits
    while cursor < len(value) and value[cursor] in allowed:
        cursor += 1
    digits = value[digits_start:cursor]
    terminated = cursor < len(value) and value[cursor] == ";"
    if terminated:
        cursor += 1
    if not digits:
        # "&#x;" is U+FFFD; "&#;", "&#xz;" and a bare "&#x" are literal.
        return (cursor - start, "\ufffd") if hex_form and terminated else (0, "")
    if not terminated and not hex_form and len(digits) < 2:
        return 0, ""
    return cursor - start, _go_rune(_accumulate(digits, 16 if hex_form else 10))


def _accumulate(digits: str, base: int) -> int:
    """The value Go's scanner accumulates, including its overflow.

    Go builds the code point in a ``rune``, which is an int32, and lets it WRAP
    -- so ``&#4294967361;`` is 0x100000041 truncated to 0x41, an ordinary "A".
    Python's int is arbitrary-precision and would call that out of range and
    emit U+FFFD instead. No corpus produces the case by accident; it takes a
    value that wraps back into the valid range.
    """
    value = 0
    for digit in digits:
        value = (value * base + int(digit, 16)) & 0xFFFFFFFF
    return value - 0x100000000 if value >= 0x80000000 else value


def _go_rune(code: int) -> str:
    if 0x80 <= code <= 0x9F:
        return _C1_REPLACEMENTS[code - 0x80]
    # `code <= 0` rather than `== 0`: the accumulator wraps into int32, so a
    # value can land negative, and Go's EncodeRune writes RuneError for those
    # exactly as its switch does for zero.
    if code <= 0 or 0xD800 <= code <= 0xDFFF or code > 0x10FFFF:
        return "\ufffd"
    return chr(code)


# --- SignedGlobalID envelopes -----------------------------------------------


class _MarshalError(ValueError):
    """A Marshal payload this reader will not decode."""


def _global_id_from_sgid(sgid: str) -> str | None:
    """The global id string an sgid's envelope carries.

    A signed sgid is ``<payload>--<digest>``, and ``-`` is a base64url
    character, so the payload itself may contain ``--``. The separator is
    therefore the LAST one, as Rails' own verifier reads it; the whole value is
    tried as a bare payload when that fails, which is what an unsigned envelope
    -- one that happens to contain ``--`` included -- needs.
    """
    value = go_trim_space(sgid)
    separator = value.rfind("--")
    if separator > 0:
        gid = _envelope_gid(value[:separator])
        if gid is not None:
            return gid
    return _envelope_gid(value)


def _envelope_gid(payload: str) -> str | None:
    """Decode one base64 payload and return the gid its envelope carries."""
    # The bound is applied to the encoded form first, so an oversized sgid costs
    # nothing to refuse -- no normalization, no decode buffer.
    if not payload or len(payload) > _MAX_SGID_ENCODED_BYTES:
        return None
    # Rails' MessageVerifier emits either alphabet; base64url is current. Both
    # decode through the standard alphabet once the two symbols are mapped, and
    # stripping the padding lets a truncated-but-valid payload through.
    normalized = payload.replace("-", "+").replace("_", "/").rstrip("=")
    # Go's base64 decoder skips CR and LF mid-stream and `validate=True` does
    # not, so a line-wrapped payload would decode there and not here. Rails
    # emits it unwrapped; this only keeps the two readers agreeing on what
    # counts as the same sgid.
    normalized = normalized.replace("\r", "").replace("\n", "")
    # ORDER matters, and this is the half that is easy to lose. Go trims the
    # padding FIRST and only then hands the value to a decoder that skips line
    # breaks, so a "=" the trim could not reach -- one with a break after it --
    # reaches `RawStdEncoding`, which has no padding character and refuses it.
    # Removing the breaks first and re-padding would quietly resolve a person
    # Go names nobody for, which is the write side's one gate.
    if "=" in normalized:
        return None
    try:
        raw = base64.b64decode(normalized + "=" * (-len(normalized) % 4), validate=True)
    except (binascii.Error, ValueError):
        return None
    if not raw or len(raw) > _MAX_SGID_PAYLOAD_BYTES:
        return None

    envelope: Any
    if raw[:2] == b"\x04\x08":
        try:
            envelope = _unmarshal_ruby(raw[2:])
        except _MarshalError:
            return None
    elif raw[:1] == b"{":
        # Decoded with replacement, because Go's encoding/json substitutes
        # U+FFFD for invalid UTF-8 inside a string and carries on; passing the
        # bytes straight to json.loads raises instead, so an envelope Go reads
        # would name nobody here.
        try:
            envelope = json.loads(raw.decode("utf-8", "replace"))
        except ValueError:
            return None
    else:
        return None

    if not isinstance(envelope, dict):
        return None
    # A SignedGlobalID is bound to a purpose, and only an "attachable" one may
    # be placed in rich text: BC3 refuses any other, so a Person sgid minted for
    # bookmarking or reading is not a mention however valid its gid. Both
    # layouts carry the purpose; an envelope without one is not a Rails
    # envelope.
    #
    # Current layout: {"_rails" => {"data" => gid, "pur" => purpose}}.
    rails = envelope.get("_rails")
    if isinstance(rails, dict):
        if rails.get("pur") != _SGID_PURPOSE_ATTACHABLE:
            return None
        gid = rails.get("data")
        return gid if isinstance(gid, str) and gid else None
    # Older layout: {"gid" => gid, "purpose" => ..., "expires_at" => ...}.
    if envelope.get("purpose") != _SGID_PURPOSE_ATTACHABLE:
        return None
    gid = envelope.get("gid")
    return gid if isinstance(gid, str) and gid else None


def _unmarshal_ruby(data: bytes) -> Any:
    """Decode the subset of Ruby's Marshal 4.8 format a SignedGlobalID payload uses.

    ``nil``, booleans, fixnums, strings (with their encoding ivars), symbols and
    symbol links, arrays and hashes, into plain Python values. Anything else
    raises :class:`_MarshalError`; the caller then treats the sgid as
    undecodable rather than guessing.
    """
    reader = _RubyMarshalReader(data)
    value = reader.value(0)
    if reader.pos != len(data):
        # A Marshal dump is exactly one value; bytes after it are corruption.
        raise _MarshalError(f"marshal: {len(data) - reader.pos} trailing bytes")
    return value


class _RubyMarshalReader:
    def __init__(self, data: bytes) -> None:
        self._data = data
        self.pos = 0
        self._symbols: list[str] = []

    def _byte(self) -> int:
        if self.pos >= len(self._data):
            raise _MarshalError("marshal: unexpected end of data")
        value = self._data[self.pos]
        self.pos += 1
        return value

    def _take(self, count: int) -> bytes:
        # The bound is checked against what REMAINS, never by adding count to
        # the position: every length reaches here through _count(), which
        # already rejected anything past the remaining bytes.
        if count < 0 or count > len(self._data) - self.pos:
            raise _MarshalError("marshal: unexpected end of data")
        raw = self._data[self.pos : self.pos + count]
        self.pos += count
        return raw

    def _int(self) -> int:
        """Marshal's packed integer.

        0 is 0; 1..4 and -1..-4 are a byte count for a little-endian value;
        anything else is the value itself offset by 5.
        """
        lead = self._byte()
        # The lead byte is a signed int8; sign-extend it arithmetically.
        signed = lead - 256 if lead > 127 else lead
        if signed == 0:
            return 0
        if signed > 4:
            return signed - 5
        if signed < -4:
            return signed + 5
        if signed > 0:
            value = 0
            for index, byte in enumerate(self._take(signed)):
                value |= byte << (8 * index)
            return value
        value = -1
        for index, byte in enumerate(self._take(-signed)):
            value &= ~(0xFF << (8 * index))
            value |= byte << (8 * index)
        return value

    def _count(self) -> int:
        """A length or count, rejecting one that cannot be honest.

        Negative, or more than the bytes left (every element takes at least one
        byte). Allocation follows what actually decodes, so a hostile count
        costs its own bytes to refuse, never the capacity it claims.
        """
        count = self._int()
        if count < 0 or count > len(self._data) - self.pos:
            raise _MarshalError(f"marshal: bad count {count}")
        return count

    def value(self, depth: int) -> Any:
        if depth > _RUBY_MARSHAL_MAX_DEPTH:
            raise _MarshalError("marshal: nesting too deep")
        tag = self._byte()
        if tag == ord("0"):
            return None
        if tag == ord("T"):
            return True
        if tag == ord("F"):
            return False
        if tag == ord("i"):
            return self._int()
        if tag == ord('"'):
            return self._take(self._count()).decode("utf-8", "surrogateescape")
        if tag == ord(":"):
            symbol = self._take(self._count()).decode("utf-8", "surrogateescape")
            self._symbols.append(symbol)
            return symbol
        if tag == ord(";"):
            index = self._int()
            if index < 0 or index >= len(self._symbols):
                raise _MarshalError("marshal: bad symbol link")
            return self._symbols[index]
        if tag == ord("I"):
            # An object followed by its instance variables -- a String's encoding.
            inner = self.value(depth + 1)
            for _ in range(self._count()):
                self.value(depth + 1)  # ivar name
                self.value(depth + 1)  # ivar value
            return inner
        if tag == ord("["):
            return [self.value(depth + 1) for _ in range(self._count())]
        if tag == ord("{"):
            out: dict[str, Any] = {}
            for _ in range(self._count()):
                key = self.value(depth + 1)
                item = self.value(depth + 1)
                if not isinstance(key, str):
                    raise _MarshalError("marshal: non-string hash key")
                out[key] = item
            return out
        raise _MarshalError(f"marshal: unsupported type {chr(tag)!r}")
