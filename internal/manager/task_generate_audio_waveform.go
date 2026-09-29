package manager

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

const (
	audioWaveformPeaks      = 1200
	audioWaveformSampleRate = 8000
)

// GenerateAudioWaveformTask builds a compact, client-ready peak cache while
// retaining the HTML audio element as the source of truth for playback.
type GenerateAudioWaveformTask struct {
	repository models.Repository
	File       *models.AudioFile
	Overwrite  bool
}

func (t *GenerateAudioWaveformTask) GetDescription() string {
	return fmt.Sprintf("Generating audio waveform for %s", t.File.Path)
}

func (t *GenerateAudioWaveformTask) required() bool {
	return t.Overwrite || len(t.File.Waveform) == 0
}

func (t *GenerateAudioWaveformTask) Start(ctx context.Context) {
	if !t.required() {
		return
	}

	peaks, err := generateAudioWaveform(ctx, t.File)
	if err != nil {
		logger.Errorf("Error generating audio waveform for %q: %v", t.File.Path, err)
		return
	}

	r := t.repository
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		return r.File.UpdateAudioWaveform(ctx, t.File.ID, peaks)
	}); err != nil && ctx.Err() == nil {
		logger.Errorf("Error saving audio waveform for %q: %v", t.File.Path, err)
	}
}

func generateAudioWaveform(ctx context.Context, file *models.AudioFile) ([]float64, error) {
	args := []string{
		"-v", "error", "-i", file.Path, "-vn", "-ac", "1",
		"-ar", fmt.Sprint(audioWaveformSampleRate), "-f", "f32le", "pipe:1",
	}
	cmd := instance.FFMpeg.Command(ctx, args)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	peaks := make([]float64, audioWaveformPeaks)
	totalSamples := int64(math.Ceil(file.DurationFinite() * audioWaveformSampleRate))
	if totalSamples < audioWaveformPeaks {
		totalSamples = audioWaveformPeaks
	}
	var sampleIndex int64
	var raw [4]byte
	for {
		_, readErr := io.ReadFull(stdout, raw[:])
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if readErr == io.ErrUnexpectedEOF {
				break
			}
			return nil, readErr
		}
		value := math.Abs(float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[:]))))
		if value > 1 {
			value = 1
		}
		bucket := int(sampleIndex * audioWaveformPeaks / totalSamples)
		if bucket >= audioWaveformPeaks {
			bucket = audioWaveformPeaks - 1
		}
		if value > peaks[bucket] {
			peaks[bucket] = value
		}
		sampleIndex++
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %w: %s", err, stderr.String())
	}
	for i := range peaks {
		peaks[i] = math.Round(peaks[i]*10000) / 10000
	}
	return peaks, nil
}
