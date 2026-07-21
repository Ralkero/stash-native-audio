from __future__ import annotations

import base64
import hashlib
import json
import os
from pathlib import Path
import re
import sys
from datetime import datetime, timezone
from typing import Any
import unicodedata
from urllib import request

PLUGIN_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(PLUGIN_DIR / "_vendor"))

from mutagen.id3 import ID3, ID3NoHeaderError  # type: ignore

PROVENANCE_KEY = "audio_metadata_import_v1"
PREVIEW_PATH = PLUGIN_DIR / "state" / "latest_preview.json"
CHOICES_PATH = PLUGIN_DIR / "state" / "conflict_choices.json"
ALLOWLIST_PATH = PLUGIN_DIR / "promotion_allowlist.json"
FIELD_NAMES = (
    "title",
    "authors",
    "album",
    "grouping",
    "genres",
    "descriptors",
    "audience",
    "content_type",
    "details",
    "original_filename",
    "cover_hash",
)


def log(level: str, message: str) -> None:
    codes = {"debug": "d", "info": "i", "warning": "w", "error": "e", "progress": "p"}
    print(f"\x01{codes[level]}\x02{message}\n", file=sys.stderr, flush=True)


def normalize_text(value: Any) -> str:
    if value is None:
        return ""
    value = unicodedata.normalize("NFC", str(value))
    return re.sub(r"\s+", " ", value).strip()


def normalized_list(values: list[Any]) -> list[str]:
    result: list[str] = []
    seen: set[str] = set()
    for raw in values:
        for value in re.split(r"\s*(?:;|\x00)\s*", normalize_text(raw)):
            value = normalize_text(value)
            key = value.casefold()
            if value and key not in seen:
                seen.add(key)
                result.append(value)
    return result


def first_text(tags: ID3, frame_id: str) -> str:
    frame = tags.get(frame_id)
    return normalize_text(frame.text[0]) if frame is not None and getattr(frame, "text", None) else ""


def txxx(tags: ID3, description: str) -> list[str]:
    wanted = description.casefold()
    values: list[Any] = []
    for frame in tags.getall("TXXX"):
        if normalize_text(frame.desc).casefold() == wanted:
            values.extend(frame.text)
    return normalized_list(values)


def comment(tags: ID3, description: str) -> str:
    wanted = description.casefold()
    candidates = []
    for frame in tags.getall("COMM"):
        desc = normalize_text(frame.desc).casefold()
        if desc == wanted or (not desc and not candidates):
            candidates.append(normalize_text(frame.text[0] if frame.text else ""))
    return next((x for x in candidates if x), "")


def cover_from_tags(tags: ID3) -> tuple[str, str]:
    covers = tags.getall("APIC")
    if not covers:
        return "", ""
    preferred = next((x for x in covers if getattr(x, "type", None) == 3), covers[0])
    raw = bytes(preferred.data)
    mime = normalize_text(getattr(preferred, "mime", "")) or "image/jpeg"
    digest = hashlib.sha256(raw).hexdigest()
    return digest, f"data:{mime};base64,{base64.b64encode(raw).decode('ascii')}"


def read_mp3(path: str) -> dict[str, Any]:
    try:
        tags = ID3(path)
    except ID3NoHeaderError:
        tags = ID3()
    artists = txxx(tags, "Stash Authors")
    if not artists:
        artists = normalized_list([first_text(tags, "TPE2") or first_text(tags, "TPE1")])
    genres = txxx(tags, "Stash Tags")
    if not genres:
        genres = normalized_list(list(getattr(tags.get("TCON"), "text", []) or []))
    cover_hash, cover_data = cover_from_tags(tags)
    original = txxx(tags, "Original Filename")
    return {
        "title": first_text(tags, "TIT2"),
        "authors": artists,
        "album": first_text(tags, "TALB"),
        "grouping": first_text(tags, "TIT1"),
        "genres": genres,
        "descriptors": txxx(tags, "Stash Descriptors"),
        "audience": (txxx(tags, "Audience") or [""])[0],
        "content_type": (txxx(tags, "Content Type") or [""])[0],
        "details": comment(tags, "Description"),
        "original_filename": original[0] if original else Path(path).name,
        "cover_hash": cover_hash,
        "_cover_data": cover_data,
    }


def file_fingerprint(path: str) -> dict[str, Any]:
    h = hashlib.sha256()
    with open(path, "rb") as source:
        while block := source.read(1024 * 1024):
            h.update(block)
    stat = os.stat(path)
    return {"size": stat.st_size, "mtime_ns": stat.st_mtime_ns, "sha256": h.hexdigest()}


def canonical(value: Any) -> Any:
    if isinstance(value, str):
        return normalize_text(value)
    if isinstance(value, list):
        return normalized_list(value)
    if isinstance(value, dict):
        return {k: canonical(value[k]) for k in sorted(value)}
    return value


def digest(value: Any) -> str:
    payload = json.dumps(canonical(value), ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


class GraphQL:
    def __init__(self, connection: dict[str, Any]):
        scheme = connection.get("Scheme", "http")
        host = connection.get("Host", "localhost") or "localhost"
        if host in ("0.0.0.0", "::", "[::]"):
            host = "127.0.0.1"
        port = connection.get("Port", 9999)
        self.url = f"{scheme}://{host}:{port}/graphql"
        cookie = connection.get("SessionCookie") or {}
        self.cookie = f"session={cookie.get('Value')}" if cookie.get("Value") else ""

    def call(self, query: str, variables: dict[str, Any] | None = None) -> dict[str, Any]:
        body = json.dumps({"query": query, "variables": variables or {}}).encode("utf-8")
        headers = {"Content-Type": "application/json", "Accept": "application/json"}
        if self.cookie:
            headers["Cookie"] = self.cookie
        req = request.Request(self.url, data=body, headers=headers, method="POST")
        with request.urlopen(req, timeout=60) as response:
            result = json.loads(response.read().decode("utf-8"))
        if result.get("errors"):
            raise RuntimeError("; ".join(x.get("message", "GraphQL error") for x in result["errors"]))
        return result.get("data") or {}


AUDIO_FIELDS = """
id title album grouping details audience content_type updated_at organized has_cover
authors { id name aliases }
tags { id name aliases }
genres descriptors custom_fields
files { id path basename size mod_time }
"""


def ensure_audio_schema(gql: GraphQL) -> None:
    data = gql.call('query { __type(name: "Audio") { name fields { name } } }')
    audio_type = data.get("__type")
    fields = {x["name"] for x in (audio_type or {}).get("fields", [])}
    required = {"id", "files", "authors", "genres", "descriptors", "custom_fields", "has_cover"}
    if not audio_type or not required.issubset(fields):
        raise RuntimeError("Compatible native Audio GraphQL support was not detected; no changes were made")


def fetch_audios(gql: GraphQL, ids: list[str] | None = None) -> list[dict[str, Any]]:
    query = f"""query Audios($ids: [ID!]) {{
      findAudios(filter: {{per_page: -1}}, ids: $ids) {{ audios {{ {AUDIO_FIELDS} }} }}
    }}"""
    return gql.call(query, {"ids": ids}).get("findAudios", {}).get("audios", [])


def fetch_groups(gql: GraphQL) -> list[dict[str, Any]]:
    query = "query { findGroups(filter: {per_page: -1}) { groups { id name aliases } } }"
    return gql.call(query).get("findGroups", {}).get("groups", [])


def fetch_tags(gql: GraphQL) -> list[dict[str, Any]]:
    query = "query { findTags(filter: {per_page: -1}) { tags { id name aliases } } }"
    return gql.call(query).get("findTags", {}).get("tags", [])


def aliases(value: Any) -> list[str]:
    if isinstance(value, list):
        return normalized_list(value)
    return normalized_list(re.split(r"[,;\n]", normalize_text(value)))


def name_index(items: list[dict[str, Any]]) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for item in items:
        for value in [item.get("name", ""), *aliases(item.get("aliases"))]:
            if normalize_text(value):
                result.setdefault(normalize_text(value).casefold(), item)
    return result


def stash_values(audio: dict[str, Any]) -> dict[str, Any]:
    custom = audio.get("custom_fields") or {}
    return {
        "title": normalize_text(audio.get("title")),
        "authors": normalized_list([x.get("name") for x in audio.get("authors") or []]),
        "album": normalize_text(audio.get("album")),
        "grouping": normalize_text(audio.get("grouping")),
        "genres": normalized_list(audio.get("genres") or []),
        "descriptors": normalized_list(audio.get("descriptors") or []),
        "audience": normalize_text(audio.get("audience")),
        "content_type": normalize_text(audio.get("content_type")),
        "details": normalize_text(audio.get("details")),
        "original_filename": normalize_text(custom.get("audio_original_filename")),
        "cover_hash": "present" if audio.get("has_cover") else "",
    }


def stash_revision(audio: dict[str, Any]) -> str:
    relevant = {k: audio.get(k) for k in (
        "title", "album", "grouping", "details", "audience", "content_type", "updated_at",
        "organized", "has_cover", "authors", "tags", "genres", "descriptors", "custom_fields", "files"
    )}
    return digest(relevant)


def read_provenance(audio: dict[str, Any]) -> dict[str, Any]:
    raw = (audio.get("custom_fields") or {}).get(PROVENANCE_KEY)
    if not raw:
        return {}
    if isinstance(raw, dict):
        return raw
    try:
        return json.loads(raw)
    except (TypeError, json.JSONDecodeError):
        return {}


def equal_values(left: Any, right: Any) -> bool:
    if isinstance(left, list) or isinstance(right, list):
        return {normalize_text(x).casefold() for x in (left or [])} == {
            normalize_text(x).casefold() for x in (right or [])
        }
    return normalize_text(left).casefold() == normalize_text(right).casefold()


def classify_fields(current: dict[str, Any], incoming: dict[str, Any], previous: dict[str, Any]) -> dict[str, Any]:
    changes: dict[str, Any] = {}
    unchanged: list[str] = []
    conflicts: dict[str, dict[str, Any]] = {}
    for field in FIELD_NAMES:
        file_value = incoming.get(field, [] if field in ("authors", "genres", "descriptors") else "")
        stash_value = current.get(field, [] if field in ("authors", "genres", "descriptors") else "")
        imported_before = field in previous
        prior_value = previous.get(field)
        if equal_values(stash_value, file_value):
            unchanged.append(field)
        elif (not imported_before and not stash_value) or (imported_before and equal_values(stash_value, prior_value)):
            changes[field] = file_value
        else:
            conflicts[field] = {"stash": stash_value, "file": file_value, "previous_import": prior_value}
    return {"changes": changes, "unchanged": unchanged, "conflicts": conflicts}


def current_values(audio: dict[str, Any], previous: dict[str, Any]) -> dict[str, Any]:
    current = stash_values(audio)
    # Stash exposes whether a cover exists, but intentionally does not send the
    # blob through GraphQL. A prior imported hash is therefore the authoritative
    # current value while the cover still exists.
    if audio.get("has_cover") and previous.get("cover_hash"):
        current["cover_hash"] = previous["cover_hash"]
    return current


def load_json(path: Path, fallback: Any) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8-sig"))
    except (OSError, json.JSONDecodeError):
        return fallback


def make_preview(gql: GraphQL, audio_ids: list[str] | None = None) -> dict[str, Any]:
    group_lookup = name_index(fetch_groups(gql))
    reports: list[dict[str, Any]] = []
    audios = fetch_audios(gql, audio_ids)
    for index, audio in enumerate(audios):
        files = audio.get("files") or []
        path = files[0].get("path") if files else ""
        item: dict[str, Any] = {"audio_id": audio["id"], "path": path}
        if not path or Path(path).suffix.casefold() != ".mp3":
            item["status"] = "skipped_not_mp3"
            reports.append(item)
            continue
        try:
            incoming = read_mp3(path)
            fingerprint = file_fingerprint(path)
        except Exception as exc:
            item.update({"status": "error", "error": str(exc)})
            reports.append(item)
            continue
        previous = read_provenance(audio).get("fields", {})
        classified = classify_fields(current_values(audio, previous), incoming, previous)
        missing = [name for name in incoming["authors"] if name.casefold() not in group_lookup]
        item.update({
            "status": "conflict" if classified["conflicts"] else "ready",
            "file": fingerprint,
            "stash_revision": stash_revision(audio),
            "incoming": {k: incoming[k] for k in FIELD_NAMES},
            "changes": classified["changes"],
            "unchanged": classified["unchanged"],
            "conflicts": classified["conflicts"],
            "proposed_authors": missing,
        })
        reports.append(item)
        log("progress", str((index + 1) / max(len(audios), 1)))
    preview = {
        "schema": 1,
        "created_at": datetime.now(timezone.utc).isoformat(),
        "items": reports,
        "summary": {
            "audios": len(audios),
            "ready": sum(x.get("status") == "ready" for x in reports),
            "conflicts": sum(x.get("status") == "conflict" for x in reports),
            "errors": sum(x.get("status") == "error" for x in reports),
        },
    }
    PREVIEW_PATH.parent.mkdir(parents=True, exist_ok=True)
    PREVIEW_PATH.write_text(json.dumps(preview, ensure_ascii=False, indent=2), encoding="utf-8")
    return preview


def create_group(gql: GraphQL, name: str) -> dict[str, Any]:
    query = "mutation($input: GroupCreateInput!) { groupCreate(input: $input) { id name aliases } }"
    return gql.call(query, {"input": {"name": name}})["groupCreate"]


def create_tag(gql: GraphQL, name: str) -> dict[str, Any]:
    query = "mutation($input: TagCreateInput!) { tagCreate(input: $input) { id name aliases } }"
    return gql.call(query, {"input": {"name": name}})["tagCreate"]


def update_audio(gql: GraphQL, update: dict[str, Any]) -> None:
    query = "mutation($input: AudioUpdateInput!) { audioUpdate(input: $input) { id updated_at } }"
    gql.call(query, {"input": update})


def apply_preview(gql: GraphQL, mode: str, approve_create_authors: bool) -> dict[str, Any]:
    preview = load_json(PREVIEW_PATH, {})
    if preview.get("schema") != 1 or not preview.get("items"):
        raise RuntimeError("No valid preview exists. Run Preview Metadata Import first")
    choices = (load_json(CHOICES_PATH, {}) or {}).get("choices", {}) if mode == "resolve" else {}
    allowlist = normalized_list((load_json(ALLOWLIST_PATH, {}) or {}).get("genre_to_shared_tag", []))
    allowset = {x.casefold() for x in allowlist}
    groups = fetch_groups(gql)
    group_lookup = name_index(groups)
    tags = fetch_tags(gql)
    tag_lookup = name_index(tags)
    applied = conflicts = rejected = skipped_unchanged = 0
    for item in preview["items"]:
        if item.get("status") not in ("ready", "conflict"):
            continue
        found = fetch_audios(gql, [str(item["audio_id"])])
        if len(found) != 1:
            rejected += 1
            continue
        audio = found[0]
        path = item["path"]
        if file_fingerprint(path) != item["file"] or stash_revision(audio) != item["stash_revision"]:
            log("warning", f"Rejected audio {item['audio_id']}: file or Stash record changed after preview")
            rejected += 1
            continue
        incoming_full = read_mp3(path)
        incoming = {k: incoming_full[k] for k in FIELD_NAMES}
        previous_provenance = read_provenance(audio)
        previous_fields = previous_provenance.get("fields", {})
        classified = classify_fields(current_values(audio, previous_fields), incoming, previous_fields)
        selected: dict[str, Any] = dict(classified["changes"])
        for field, conflict in classified["conflicts"].items():
            choice = choices.get(f"{audio['id']}.{field}")
            if choice == "use_file":
                selected[field] = conflict["file"]
            elif choice not in ("keep_stash", None):
                raise RuntimeError(f"Invalid conflict choice for {audio['id']}.{field}: {choice}")
            else:
                conflicts += 1
        update: dict[str, Any] = {"id": audio["id"]}
        scalar_map = {
            "title": "title", "album": "album", "grouping": "grouping", "audience": "audience",
            "content_type": "content_type", "details": "details",
        }
        imported_fields = dict(previous_fields)
        for source, destination in scalar_map.items():
            if source in selected:
                update[destination] = selected[source] or None
                imported_fields[source] = selected[source]
        if "authors" in selected:
            author_ids: list[str] = []
            for author_name in selected["authors"]:
                author = group_lookup.get(author_name.casefold())
                if author is None:
                    if not approve_create_authors:
                        raise RuntimeError(f"Author '{author_name}' requires explicit creation approval")
                    author = create_group(gql, author_name)
                    group_lookup = name_index([*groups, author])
                    groups.append(author)
                author_ids.append(author["id"])
            update["author_ids"] = author_ids
            imported_fields["authors"] = selected["authors"]
        for source in ("genres", "descriptors"):
            if source in selected:
                update[source] = selected[source]
                imported_fields[source] = selected[source]
        if "cover_hash" in selected:
            update["cover_image"] = incoming_full["_cover_data"] or ""
            imported_fields["cover_hash"] = selected["cover_hash"]
        custom_partial: dict[str, Any] = {}
        if "original_filename" in selected:
            custom_partial["audio_original_filename"] = selected["original_filename"]
            imported_fields["original_filename"] = selected["original_filename"]
        promoted_ids = [x["id"] for x in (audio.get("tags") or [])]
        for genre in incoming["genres"]:
            if genre.casefold() not in allowset:
                continue
            tag = tag_lookup.get(genre.casefold())
            if tag is None:
                tag = create_tag(gql, genre)
                tags.append(tag)
                tag_lookup = name_index(tags)
            if tag["id"] not in promoted_ids:
                promoted_ids.append(tag["id"])
        tag_changes = promoted_ids != [x["id"] for x in (audio.get("tags") or [])]
        if tag_changes:
            update["tag_ids"] = promoted_ids
        if not selected and not tag_changes and previous_provenance and not classified["conflicts"]:
            skipped_unchanged += 1
            continue
        for field in classified["unchanged"]:
            imported_fields[field] = incoming[field]
        provenance = {
            "schema": 1,
            "source": "embedded-id3",
            "imported_at": datetime.now(timezone.utc).isoformat(),
            "file": file_fingerprint(path),
            "fields": imported_fields,
            "field_hashes": {key: digest(value) for key, value in imported_fields.items()},
        }
        custom_partial[PROVENANCE_KEY] = json.dumps(provenance, ensure_ascii=False, sort_keys=True)
        update["custom_fields"] = {"partial": custom_partial}
        update_audio(gql, update)
        applied += 1
    return {"applied": applied, "skipped_unchanged": skipped_unchanged, "unresolved_conflicts": conflicts, "rejected": rejected}


def main() -> None:
    plugin_input = json.loads(sys.stdin.read() or "{}")
    args = plugin_input.get("args") or {}
    mode = normalize_text(args.get("mode") or "preview").casefold()
    gql = GraphQL(plugin_input.get("server_connection") or {})
    ensure_audio_schema(gql)
    if mode == "preview":
        ids = [str(x) for x in args.get("audio_ids", [])] or None
        result = make_preview(gql, ids)
        output = {"output": f"Preview complete: {result['summary']}. Review {PREVIEW_PATH}"}
    elif mode in ("apply", "resolve"):
        result = apply_preview(gql, mode, bool(args.get("approve_create_authors", mode == "apply")))
        output = {"output": f"Import complete: {result}"}
    else:
        raise RuntimeError(f"Unknown mode: {mode}")
    print(json.dumps(output, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        log("error", str(exc))
        print(json.dumps({"error": str(exc)}, ensure_ascii=False))
        raise SystemExit(1)
