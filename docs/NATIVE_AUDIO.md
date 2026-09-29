# Native Audio for Stash

This experimental fork adds MP3 audio as a first-class Stash entity without wrapping audio files as scenes or videos. It is based on Stash v0.31.1.

## Included functionality

- `Audio` and `AudioFile` database models, with schema 86 for native audio and schema 87 for cached waveform peaks
- MP3 scanning with duration, codec, bitrate, sample rate, channels, path, and hash data
- Audio metadata fields for title, authors, album, grouping, genres, descriptors, audience, content type, description, cover art, rating, organized state, play count, and resume position
- GraphQL CRUD, search, filtering, sorting, pagination, playback activity, configuration, and capability detection
- Native `/audios` browse and detail routes with a WaveSurfer.js v7 waveform attached to the existing HTML audio element
- Interactive seeking, timeline and hover labels, transport, volume, mute, speed, loop, loading/error states, and native-control fallback
- On-demand **Audio waveforms** generation under Settings > Tasks > Generate
- Optional embedded-ID3 metadata importer in `contrib/embedded-audio-metadata`

Authors reuse Stash Groups. Albums remain an audio field rather than becoming Galleries, and audio records do not enter scene-only pHash or video-variant workflows.

## Safety and enablement

The feature is disabled by default. Before enabling it:

1. Stop Stash and make a verified copy of the database.
2. Retain the official v0.31.1 executable for rollback.
3. Start this build and allow migration through schema 87.
4. Enable **Audio library** in Settings > Library.
5. Confirm `audio_extensions` contains only `mp3` for the v1 implementation.
6. Scan the configured library paths.

Do not run an older executable against a database already migrated to schema 87.

## Embedded metadata plugin

The companion plugin reads ID3 metadata with Mutagen, previews changes, and applies approved values through GraphQL. It never writes directly to the Stash database or back into media files.

To prepare the source directory for installation:

```powershell
cd contrib/embedded-audio-metadata
python -m pip install --target _vendor -r requirements.txt
```

Copy the completed directory to the Stash plugin folder, reload plugins, and run **Preview Metadata Import** before **Apply Reviewed Import**.

## Development checks

Relevant validation includes:

```powershell
go test ./pkg/audio ./pkg/file/audio ./pkg/sqlite ./internal/manager ./internal/api
cd ui/v2.5
pnpm run check
pnpm run build
```

The metadata importer tests can be run from its directory with:

```powershell
python -m unittest test_embedded_audio_metadata.py
```

## Rollback

Stop the custom build, restore the retained executable and its matching pre-migration database as a pair, then restart Stash.
