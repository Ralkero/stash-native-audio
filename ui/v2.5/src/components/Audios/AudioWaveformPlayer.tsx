import React, { useEffect, useRef, useState } from "react";
import { Button } from "react-bootstrap";
import WaveSurfer from "wavesurfer.js";
// eslint-disable-next-line import/extensions
import HoverPlugin from "wavesurfer.js/dist/plugins/hover.js";
// eslint-disable-next-line import/extensions
import TimelinePlugin from "wavesurfer.js/dist/plugins/timeline.js";

let activeAudio: HTMLAudioElement | null = null;

function formatTime(seconds: number) {
  if (!Number.isFinite(seconds)) return "0:00";
  const minutes = Math.floor(seconds / 60);
  return `${minutes}:${Math.floor(seconds % 60)
    .toString()
    .padStart(2, "0")}`;
}

interface IAudioWaveformPlayer {
  src: string;
  compact?: boolean;
  durationHint?: number;
  peaks?: number[];
  autoPlay?: boolean;
  loop?: boolean;
  initialTime?: number;
  onPlay?: (player: HTMLAudioElement) => void;
  onTimeUpdate?: (player: HTMLAudioElement) => void;
  onPause?: (player: HTMLAudioElement) => void;
  onEnded?: (player: HTMLAudioElement) => void;
}

export const AudioWaveformPlayer: React.FC<IAudioWaveformPlayer> = (props) => {
  const {
    src,
    compact = false,
    durationHint = 0,
    peaks,
    autoPlay = false,
    loop = false,
    initialTime = 0,
  } = props;
  const hostRef = useRef<HTMLDivElement>(null);
  const timelineRef = useRef<HTMLDivElement>(null);
  const audioRef = useRef<HTMLAudioElement>(null);
  const waveSurferRef = useRef<WaveSurfer>();
  const callbacksRef = useRef(props);
  const [ready, setReady] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [fallback, setFallback] = useState(false);
  const [status, setStatus] = useState("Loading waveform…");
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(durationHint);
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);
  const [speed, setSpeed] = useState(1);
  callbacksRef.current = props;

  useEffect(() => {
    const audio = audioRef.current;
    const host = hostRef.current;
    if (!audio || !host) return undefined;

    let disposed = false;
    setReady(false);
    setFallback(false);
    setStatus(
      peaks?.length ? "Loading cached waveform…" : "Generating waveform…"
    );
    audio.src = src;
    audio.preload = "metadata";

    const initialise = () => {
      if (disposed || waveSurferRef.current) return;
      try {
        const plugins = [
          HoverPlugin.create({
            lineColor: "#f8f7ff",
            lineWidth: 1,
            labelBackground: "#111827",
            labelColor: "#f7f8fc",
            labelSize: "11px",
            formatTimeCallback: formatTime,
          }),
        ];
        if (!compact && timelineRef.current) {
          plugins.push(
            TimelinePlugin.create({
              container: timelineRef.current,
              height: 18,
              formatTimeCallback: formatTime,
              style: {
                color: "#aac0d1",
                fontFamily: "inherit",
                fontSize: "10px",
              },
            }) as never
          );
        }
        const waveSurfer = WaveSurfer.create({
          container: host,
          media: audio,
          url: peaks?.length ? "" : src,
          peaks: peaks?.length ? [peaks] : undefined,
          duration: peaks?.length ? durationHint : undefined,
          height: compact ? 52 : 84,
          waveColor: "#527391",
          progressColor: "#55b8ff",
          cursorColor: "#f8f7ff",
          cursorWidth: 2,
          barWidth: 2,
          barGap: 1,
          barRadius: 2,
          normalize: true,
          dragToSeek: true,
          interact: true,
          hideScrollbar: true,
          plugins,
        });
        waveSurferRef.current = waveSurfer;
        waveSurfer.on("ready", (value) => {
          if (disposed) return;
          setDuration(value || durationHint);
          setReady(true);
          setStatus("");
          if (initialTime > 0 && initialTime < value - 2) {
            audio.currentTime = initialTime;
          }
          if (autoPlay) void audio.play();
        });
        waveSurfer.on("loading", (percent) =>
          setStatus(`Loading waveform… ${percent}%`)
        );
        waveSurfer.on("error", () => {
          setFallback(true);
          setStatus("Waveform unavailable. Native audio controls are active.");
        });
      } catch {
        setFallback(true);
        setStatus("Waveform unavailable. Native audio controls are active.");
      }
    };

    let observer: IntersectionObserver | undefined;
    if (compact && "IntersectionObserver" in window) {
      observer = new IntersectionObserver(
        (entries) => {
          if (entries.some((entry) => entry.isIntersecting)) {
            initialise();
            observer?.disconnect();
          }
        },
        { rootMargin: "240px" }
      );
      observer.observe(host);
    } else {
      initialise();
    }

    const onPlay = () => {
      if (activeAudio && activeAudio !== audio) activeAudio.pause();
      activeAudio = audio;
      setPlaying(true);
      callbacksRef.current.onPlay?.(audio);
    };
    const onPause = () => {
      setPlaying(false);
      callbacksRef.current.onPause?.(audio);
    };
    const onTimeUpdate = () => {
      setCurrentTime(audio.currentTime);
      callbacksRef.current.onTimeUpdate?.(audio);
    };
    const onDurationChange = () => setDuration(audio.duration || durationHint);
    const onWaiting = () => setStatus("Buffering…");
    const onPlaying = () => setStatus("");
    const onEnded = () => callbacksRef.current.onEnded?.(audio);
    const onError = () => {
      setFallback(true);
      setStatus(
        audio.error?.code === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED
          ? "Unsupported audio format."
          : "Audio playback error."
      );
    };
    audio.addEventListener("play", onPlay);
    audio.addEventListener("pause", onPause);
    audio.addEventListener("timeupdate", onTimeUpdate);
    audio.addEventListener("durationchange", onDurationChange);
    audio.addEventListener("waiting", onWaiting);
    audio.addEventListener("playing", onPlaying);
    audio.addEventListener("ended", onEnded);
    audio.addEventListener("error", onError);

    return () => {
      disposed = true;
      observer?.disconnect();
      audio.pause();
      audio.removeEventListener("play", onPlay);
      audio.removeEventListener("pause", onPause);
      audio.removeEventListener("timeupdate", onTimeUpdate);
      audio.removeEventListener("durationchange", onDurationChange);
      audio.removeEventListener("waiting", onWaiting);
      audio.removeEventListener("playing", onPlaying);
      audio.removeEventListener("ended", onEnded);
      audio.removeEventListener("error", onError);
      if (activeAudio === audio) activeAudio = null;
      waveSurferRef.current?.destroy();
      waveSurferRef.current = undefined;
      audio.removeAttribute("src");
      audio.load();
    };
  }, [autoPlay, compact, durationHint, initialTime, peaks, src]);

  useEffect(() => {
    if (audioRef.current) audioRef.current.loop = loop;
  }, [loop]);

  const seekBy = (seconds: number) => {
    const audio = audioRef.current;
    if (audio)
      audio.currentTime = Math.max(
        0,
        Math.min(audio.duration, audio.currentTime + seconds)
      );
  };

  return (
    <div className={`audio-waveform-player${compact ? " is-compact" : ""}`}>
      <audio
        ref={audioRef}
        controls={fallback}
        className={fallback ? "audio-native-fallback" : "d-none"}
      />
      {!fallback && (
        <>
          <div className="audio-waveform-stage" ref={hostRef} />
          {!compact && (
            <div className="audio-waveform-timeline" ref={timelineRef} />
          )}
          {status && <div className="audio-waveform-status">{status}</div>}
          <div className="audio-waveform-controls">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => seekBy(-10)}
              aria-label="Rewind 10 seconds"
            >
              −10
            </Button>
            <Button
              size="sm"
              onClick={() =>
                playing
                  ? audioRef.current?.pause()
                  : void audioRef.current?.play()
              }
              disabled={!ready}
            >
              {playing ? "Pause" : "Play"}
            </Button>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => seekBy(10)}
              aria-label="Forward 10 seconds"
            >
              +10
            </Button>
            <span className="audio-waveform-time">
              {formatTime(currentTime)} / {formatTime(duration)}
            </span>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                const next = !muted;
                setMuted(next);
                if (audioRef.current) audioRef.current.muted = next;
              }}
            >
              {muted ? "Unmute" : "Volume"}
            </Button>
            <input
              aria-label="Volume"
              type="range"
              min="0"
              max="1"
              step="0.01"
              value={volume}
              onChange={(event) => {
                const next = Number(event.currentTarget.value);
                setVolume(next);
                if (audioRef.current) audioRef.current.volume = next;
              }}
            />
            <select
              aria-label="Playback speed"
              value={speed}
              onChange={(event) => {
                const next = Number(event.currentTarget.value);
                setSpeed(next);
                if (audioRef.current) audioRef.current.playbackRate = next;
              }}
            >
              {[0.5, 0.75, 1, 1.25, 1.5, 2].map((value) => (
                <option value={value} key={value}>
                  {value}×
                </option>
              ))}
            </select>
          </div>
        </>
      )}
    </div>
  );
};
