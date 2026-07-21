# Embedded Audio Metadata

This Stash raw-process plugin imports embedded MP3 metadata into the native
`Audio` GraphQL entity. It never writes to media files or directly to the
Stash database.

Run **Preview Metadata Import** first. The report is written to
`state/latest_preview.json`. Running **Apply Reviewed Import** is the explicit
approval action; it is rejected if either a file or its Stash record changed
after preview.

Conflicts are left untouched. To resolve them, copy the proposed choices into
`state/conflict_choices.json` using keys in the form `audio-id.field` and a
value of either `keep_stash` or `use_file`, then run **Resolve Conflicts**.

Genre promotion is disabled by default. Add exact genre names to
`promotion_allowlist.json` only when those values should also become ordinary
Stash Shared Tags. Promotions are additive and never remove existing tags.

Before copying this source directory into Stash, install the pinned Mutagen
dependency locally:

```powershell
python -m pip install --target _vendor -r requirements.txt
```

Release packages may include that `_vendor` directory so no system-wide Python
package is needed. The plugin fails closed when the server does not expose the
native `Audio` GraphQL type.
