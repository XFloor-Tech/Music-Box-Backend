## Stream format

The current streaming model is a small custom subset of MPEG-DASH for AAC audio
in fragmented MP4 (fMP4/CMAF-style) containers. The player reads a static
`manifest.mpd` file, parses the DASH `SegmentTemplate` and `SegmentTimeline`,
and uses that expanded segment index to fetch `init.mp4` and `.m4s` media
segments.

This is not HLS. HLS would use an `.m3u8` playlist. The `.m4s` media segments
and `init.mp4` initialization segment are fMP4/CMAF-style assets that can be
used by both DASH and HLS ecosystems, but the manifest format this project
currently parses is DASH MPD.

The `init.mp4` is the initialization segment containing essential headers and
metadata (e.g., codec configuration via the AudioSpecificConfig in the `moov`
box), while the `.m4s` files are media segments holding the actual audio data in
fragmented form (`moof` and `mdat` boxes).

Each .m4s chunk is self-sustained in the context of sequential streaming once the decoder is initialized with the init.mp4 data:

- No keyframes needed.
- The .m4s segments contain complete audio samples (raw AAC frames) prefixed by length info from the 'trun' box in 'moof'. As long as you demux the raw bitstream from the .m4s (extracting from 'mdat' using offsets/sizes from 'moof'), you can decode it immediately after init. The segments are designed for concatenation and sequential decoding in streaming scenarios.
- The only dependency is the initial configuration from init.mp4 (e.g., sample rate, channels, profile via AudioSpecificConfig). Without that, isolated .m4s chunks wouldn't know the codec params.

## Stream cache model

The audio cache stores downloaded stream bytes: the init.mp4 response metadata
and the .m4s segment buffers. It does not currently store decoded PCM frames or
ring buffer contents.

This keeps the cache as a recoverable source of truth. If playback needs to be
restarted, replayed, or rebuilt after ring buffer state becomes invalid, the
player can create a fresh demuxer/decoder path from the cached media bytes. Raw
MP4 bytes are also much smaller and less tied to the current render format than
decoded Float32 PCM.

Because cached bytes are not playable by themselves, cached playback still
demuxes and decodes before audio becomes loaded into the ring buffer. In the
current player, loaded duration means decoded PCM frames written to the ring
buffer, not bytes downloaded or available in cache. A track can therefore be
fully cached/downloaded while loaded duration still grows during cached
demux/decode.

A future processed cache may store decoded interleaved PCM for short tracks or
valid replay windows, but it should be a separate layer with format metadata and
size limits. When processed cache is unavailable, stale, too large, or
incompatible with the current render format, the player should fall back to the
raw downloaded byte cache and demux/decode again.

## Transcode file into AAC in MP4 container using CLI

Convert to AAC:

- ffmpeg -i input.mp3 -vn -c:a aac -b:a 256k output.mp4

Make `.m4s` segments and `init.mp4` along with a DASH `manifest.mpd`:

- MP4Box -dash 10000 -frag 2000 -rap -profile live -segment-timeline -url-template -segment-name '$Init=init$:$Segment=$Number$.m4s' -out manifest.mpd output.mp4

With `-segment-timeline`, MP4Box may write the media segment template as
`$Time$.m4s` even when the requested segment name uses `$Number$`. In that
case, the generated files are named by each segment's start time in the
`SegmentTemplate` timescale:

- `manifest.mpd`
- `init.mp4`
- `0.m4s`, `440320.m4s`, `881664.m4s`, ...

The important addition is `-segment-timeline`. It makes MP4Box write a
`SegmentTimeline` inside the MPD's `SegmentTemplate`, so the player can derive
exact segment start times and durations from the manifest instead of assuming
every segment has the same duration. The `timescale` on the `SegmentTemplate`
defines the units used by each timeline entry and by `$Time$` segment names.

The expected MPD shape is:

```xml
<SegmentTemplate media="$Time$.m4s" initialization="init.mp4" timescale="44100" startNumber="1">
  <SegmentTimeline>
    <S t="0" d="440320" />
    <S d="441344" />
    <S d="440320" />
    <S d="441344" r="1" />
    <S d="328704" />
  </SegmentTimeline>
</SegmentTemplate>
```

Actual `d`, `r`, and optional `t` values depend on the source track and encoder
padding. For seek indexing, compute each segment's start by expanding the
timeline in order, using `t` when present and otherwise continuing from the
previous entry end. When the `media` template contains `$Time$`, substitute that
computed start value to fetch the segment URL.

## Fetching segments from static storage

When `manifest.mpd`, `init.mp4`, and `.m4s` files are stored in static object
storage such as an R2 bucket, the client does not need to list the bucket or
guess filenames. The manifest is the segment index source of truth.

For a static track directory like:

```txt
tracks/176/
  manifest.mpd
  init.mp4
  0.m4s
  440320.m4s
  881664.m4s
```

the fetch flow is:

1. Fetch `tracks/176/manifest.mpd`.
2. Parse the `SegmentTemplate` to get `initialization`, `media`, `timescale`,
   and `startNumber`.
3. Expand the `SegmentTimeline` in order into an array of segment records.
4. Fetch `tracks/176/init.mp4` from the `initialization` value.
5. Fetch each media segment by substituting the expanded segment values into the
   `media` template.

For example, with:

```xml
<SegmentTemplate media="$Time$.m4s" initialization="init.mp4" timescale="44100" startNumber="1">
  <SegmentTimeline>
    <S t="0" d="440320" />
    <S d="441344" />
    <S d="440320" />
  </SegmentTimeline>
</SegmentTemplate>
```

the expanded segment index is:

```ts
[
  { part: 1, time: 0, start: 0, duration: 440320 / 44100, url: '0.m4s' },
  {
    part: 2,
    time: 440320,
    start: 440320 / 44100,
    duration: 441344 / 44100,
    url: '440320.m4s',
  },
  {
    part: 3,
    time: 881664,
    start: 881664 / 44100,
    duration: 440320 / 44100,
    url: '881664.m4s',
  },
];
```

The next segment to fetch is simply the next item in this expanded array. For
normal sequential playback, start with `segments[0]` and continue forward. For
seeking, find the segment where `seekTime >= segment.start` and
`seekTime < segment.start + segment.duration`, then continue fetching from that
segment onward.

If the manifest uses `$Number$` instead of `$Time$`, substitute the segment
`part` value into the media template instead. For example, `part: 3` with
`media="$Number$.m4s"` resolves to `3.m4s`.
