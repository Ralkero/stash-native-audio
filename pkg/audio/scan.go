package audio

import (
	"context"
	"fmt"
	"time"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type ScanCreatorUpdater interface {
	FindByFileID(ctx context.Context, fileID models.FileID) ([]*models.Audio, error)
	Create(ctx context.Context, audio *models.Audio, fileIDs []models.FileID) error
	Update(ctx context.Context, audio *models.Audio) error
}

type ScanHandler struct{ CreatorUpdater ScanCreatorUpdater }

func (h *ScanHandler) Handle(ctx context.Context, f models.File, oldFile models.File) error {
	audioFile, ok := f.(*models.AudioFile)
	if !ok {
		return fmt.Errorf("not an audio file: %T", f)
	}
	existing, err := h.CreatorUpdater.FindByFileID(ctx, audioFile.ID)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		a := models.NewAudio()
		logger.Infof("%s doesn't exist. Creating native audio item...", audioFile.Path)
		return h.CreatorUpdater.Create(ctx, &a, []models.FileID{audioFile.ID})
	}
	if oldFile != nil {
		for _, a := range existing {
			a.UpdatedAt = time.Now()
			if err := h.CreatorUpdater.Update(ctx, a); err != nil {
				return err
			}
		}
	}
	return nil
}
