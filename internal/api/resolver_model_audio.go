package api

import (
	"context"

	"github.com/stashapp/stash/internal/api/urlbuilders"
	"github.com/stashapp/stash/pkg/models"
)

func (r *audioResolver) Files(ctx context.Context, obj *models.Audio) ([]*AudioFile, error) {
	var files []*models.AudioFile
	err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		files, err = r.repository.Audio.GetFiles(ctx, obj.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	ret := make([]*AudioFile, len(files))
	for i, f := range files {
		ret[i] = &AudioFile{AudioFile: f}
	}
	return ret, nil
}

func (r *audioResolver) Paths(ctx context.Context, obj *models.Audio) (*AudioPaths, error) {
	baseURL, _ := ctx.Value(BaseURLCtxKey).(string)
	b := urlbuilders.NewAudioURLBuilder(baseURL, obj)
	stream, cover := b.GetStreamURL(), b.GetCoverURL()
	return &AudioPaths{Stream: stream, Cover: &cover}, nil
}

func (r *audioResolver) HasCover(ctx context.Context, obj *models.Audio) (ret bool, err error) {
	err = r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Audio.HasCover(ctx, obj.ID)
		return err
	})
	return ret, err
}
func (r *audioResolver) Rating100(ctx context.Context, obj *models.Audio) (*int, error) {
	return obj.Rating, nil
}

func (r *audioResolver) Authors(ctx context.Context, obj *models.Audio) (ret []*models.Group, err error) {
	err = r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, err := r.repository.Audio.GetAuthorIDs(ctx, obj.ID)
		if err != nil {
			return err
		}
		ret, err = r.repository.Group.FindMany(ctx, ids)
		return err
	})
	return ret, err
}

func (r *audioResolver) Tags(ctx context.Context, obj *models.Audio) (ret []*models.Tag, err error) {
	err = r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, err := r.repository.Audio.GetTagIDs(ctx, obj.ID)
		if err != nil {
			return err
		}
		ret, err = r.repository.Tag.FindMany(ctx, ids)
		return err
	})
	return ret, err
}

func (r *audioResolver) Genres(ctx context.Context, obj *models.Audio) (ret []string, err error) {
	err = r.withReadTxn(ctx, func(ctx context.Context) error { ret, err = r.repository.Audio.GetGenres(ctx, obj.ID); return err })
	return ret, err
}
func (r *audioResolver) Descriptors(ctx context.Context, obj *models.Audio) (ret []string, err error) {
	err = r.withReadTxn(ctx, func(ctx context.Context) error { ret, err = r.repository.Audio.GetDescriptors(ctx, obj.ID); return err })
	return ret, err
}
func (r *audioResolver) CustomFields(ctx context.Context, obj *models.Audio) (ret map[string]interface{}, err error) {
	err = r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Audio.GetCustomFields(ctx, obj.ID)
		return err
	})
	return ret, err
}

