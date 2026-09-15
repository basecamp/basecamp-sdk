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
import html
import json
from collections.abc import Iterable, Mapping
from typing import Any
from urllib.parse import urlparse

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

#: Bounds the decoded sgid payload. A Person sgid's payload is under 200 bytes;
#: the cap keeps a hostile one from costing more than its own size to reject.
_MAX_SGID_PAYLOAD_BYTES = 4096
#: The same bound on the base64 form (4/3 of the payload, plus padding), checked
#: before anything is allocated.
_MAX_SGID_ENCODED_BYTES = _MAX_SGID_PAYLOAD_BYTES // 3 * 4 + 4

#: Bounds nesting in a Marshal payload; an envelope is two deep.
_RUBY_MARSHAL_MAX_DEPTH = 32

_SPACE = " \t\n\r\f"


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
    try:
        parsed = urlparse(gid)
    except ValueError:
        return None
    if parsed.scheme != "gid" or not parsed.netloc:
        return None
    # A GlobalID path is exactly "/<Model>/<id>": no more, no less.
    model, separator, raw_id = parsed.path.removeprefix("/").partition("/")
    if separator != "/" or model != "Person" or not raw_id:
        return None
    # `str.isdigit` is true for non-ASCII digits, which `int()` would then
    # happily parse into an id BC3 never wrote.
    if not (raw_id.isascii() and raw_id.isdigit()):
        return None
    person_id = int(raw_id)
    return person_id if person_id > 0 else None


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
    person_id = person.get("id")
    if not isinstance(person_id, int) or isinstance(person_id, bool) or person_id <= 0:
        raise UsageError(
            "cannot mention a person carrying no id",
            hint="read the person through people.get to obtain one",
        )
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
            sgid = html.unescape(value)
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
    value = sgid.strip()
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
        try:
            envelope = json.loads(raw)
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
