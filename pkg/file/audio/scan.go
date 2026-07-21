package audio

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/models"
)

// Decorator probes the technical properties of native audio files.
type Decorator struct{ FFProbe *ffmpeg.FFProbe }

func (d *Decorator) Decorate(ctx context.Context, fs models.FS, f models.File) (models.File, error) {
	if d.FFProbe == nil {
		return f, errors.New("ffprobe not configured")
	}
	if _, ok := fs.(*file.OsFS); !ok {
		return f, errors.New("native audio in zip files is not supported")
	}
	probe, err := d.FFProbe.NewVideoFile(f.Base().Path)
	if err != nil {
		return f, fmt.Errorf("probing audio %q: %w", f.Base().Path, err)
	}
	if probe.AudioStream == nil {
		return f, fmt.Errorf("%q has no audio stream", f.Base().Path)
	}
	stream := probe.AudioStream
	sampleRate, _ := strconv.Atoi(stream.SampleRate)
	bitDepth := stream.BitsPerSample
	if bitDepth == 0 {
		bitDepth, _ = strconv.Atoi(stream.BitsPerRawSample)
	}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(f.Base().Path)), ".")
	if format == "" {
		format = strings.Split(probe.Container, ",")[0]
	}
	return &models.AudioFile{
		BaseFile: f.Base(), Format: format, Duration: probe.FileDuration,
		AudioCodec: probe.AudioCodec, BitRate: probe.Bitrate,
		SampleRate: sampleRate, Channels: stream.Channels, BitDepth: bitDepth,
	}, nil
}

func (d *Decorator) IsMissingMetadata(ctx context.Context, fs models.FS, f models.File) bool {
	a, ok := f.(*models.AudioFile)
	return !ok || a.Format == "" || a.AudioCodec == "" || a.Duration < 0 || a.SampleRate <= 0 || a.Channels <= 0
}
