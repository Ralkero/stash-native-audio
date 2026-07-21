package urlbuilders

import (
	"github.com/stashapp/stash/pkg/models"
	"strconv"
)

type AudioURLBuilder struct{ BaseURL, AudioID, UpdatedAt string }

func NewAudioURLBuilder(baseURL string, audio *models.Audio) AudioURLBuilder {
	return AudioURLBuilder{BaseURL: baseURL, AudioID: strconv.Itoa(audio.ID), UpdatedAt: strconv.FormatInt(audio.UpdatedAt.Unix(), 10)}
}
func (b AudioURLBuilder) GetStreamURL() string {
	return b.BaseURL + "/audio/" + b.AudioID + "/stream?t=" + b.UpdatedAt
}
func (b AudioURLBuilder) GetCoverURL() string {
	return b.BaseURL + "/audio/" + b.AudioID + "/cover?t=" + b.UpdatedAt
}
