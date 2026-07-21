package api

import (
	"context"
	"fmt"
	"strconv"

	"github.com/99designs/gqlgen/graphql"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/utils"
)

func parseAudioIDs(values []string) ([]int, error) {
	ret := make([]int, len(values))
	for i, value := range values {
		id, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		ret[i] = id
	}
	return ret, nil
}

func parseAudioFileIDs(values []string) ([]models.FileID, error) {
	ids, err := parseAudioIDs(values)
	if err != nil {
		return nil, err
	}
	return models.FileIDsFromInts(ids), nil
}

func (r *queryResolver) FindAudio(ctx context.Context, id string) (ret *models.Audio, err error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	err = r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Audio.Find(ctx, idInt)
		return err
	})
	return ret, err
}

func (r *queryResolver) FindAudios(ctx context.Context, audioFilter *models.AudioFilterType, filter *models.FindFilterType, ids []string) (ret *FindAudiosResultType, err error) {
	err = r.withReadTxn(ctx, func(ctx context.Context) error {
		var result *models.AudioQueryResult
		if len(ids) > 0 {
			parsed, err := parseAudioIDs(ids)
			if err != nil {
				return err
			}
			audios, err := r.repository.Audio.FindMany(ctx, parsed)
			if err != nil {
				return err
			}
			result = models.NewAudioQueryResult(r.repository.Audio)
			result.IDs, result.Count = parsed, len(audios)
		} else {
			fields := graphql.CollectAllFields(ctx)
			result, err = r.repository.Audio.Query(ctx, models.AudioQueryOptions{
				QueryOptions:  models.QueryOptions{FindFilter: filter, Count: true},
				AudioFilter:   audioFilter,
				TotalDuration: containsString(fields, "duration"),
				TotalSize:     containsString(fields, "filesize"),
			})
			if err != nil {
				return err
			}
		}
		audios, err := result.Resolve(ctx)
		if err != nil {
			return err
		}
		ret = &FindAudiosResultType{Count: result.Count, Duration: result.TotalDuration, Filesize: result.TotalSize, Audios: audios}
		return nil
	})
	return ret, err
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (r *mutationResolver) AudioCreate(ctx context.Context, input models.AudioCreateInput) (ret *models.Audio, err error) {
	fileIDs, err := parseAudioFileIDs(input.FileIds)
	if err != nil {
		return nil, fmt.Errorf("file ids: %w", err)
	}
	authorIDs, err := parseAudioIDs(input.AuthorIds)
	if err != nil {
		return nil, fmt.Errorf("author ids: %w", err)
	}
	tagIDs, err := parseAudioIDs(input.TagIds)
	if err != nil {
		return nil, fmt.Errorf("tag ids: %w", err)
	}
	a := models.NewAudio()
	if input.Title != nil {
		a.Title = *input.Title
	}
	if input.Album != nil {
		a.Album = *input.Album
	}
	if input.Grouping != nil {
		a.Grouping = *input.Grouping
	}
	if input.Details != nil {
		a.Details = *input.Details
	}
	if input.Audience != nil {
		a.Audience = *input.Audience
	}
	if input.ContentType != nil {
		a.ContentType = *input.ContentType
	}
	a.Rating, a.Organized = input.Rating100, input.Organized != nil && *input.Organized
	a.AuthorIDs, a.TagIDs = models.NewRelatedIDs(authorIDs), models.NewRelatedIDs(tagIDs)
	a.Genres, a.Descriptors = input.Genres, input.Descriptors
	a.CustomFields = convertMapJSONNumbers(input.CustomFields)
	var cover []byte
	if input.CoverImage != nil {
		cover, err = utils.ProcessImageInput(ctx, *input.CoverImage)
		if err != nil {
			return nil, err
		}
	}
	err = r.withTxn(ctx, func(ctx context.Context) error {
		for _, fileID := range fileIDs {
			files, err := r.repository.File.Find(ctx, fileID)
			if err != nil {
				return err
			}
			if _, ok := files[0].(*models.AudioFile); !ok {
				return fmt.Errorf("file %d is not an AudioFile", fileID)
			}
		}
		if err := r.repository.Audio.Create(ctx, &a, fileIDs); err != nil {
			return err
		}
		if len(cover) > 0 {
			if err := r.repository.Audio.UpdateCover(ctx, a.ID, cover); err != nil {
				return err
			}
		}
		ret, err = r.repository.Audio.Find(ctx, a.ID)
		return err
	})
	return ret, err
}

func (r *mutationResolver) AudioUpdate(ctx context.Context, input models.AudioUpdateInput) (ret *models.Audio, err error) {
	id, err := strconv.Atoi(input.ID)
	if err != nil {
		return nil, err
	}
	fields := getUpdateInputMap(ctx)
	has := func(name string) bool { _, ok := fields[name]; return ok }
	err = r.withTxn(ctx, func(ctx context.Context) error {
		a, err := r.repository.Audio.Find(ctx, id)
		if err != nil || a == nil {
			return err
		}
		if has("title") {
			if input.Title == nil {
				a.Title = ""
			} else {
				a.Title = *input.Title
			}
		}
		if has("album") {
			if input.Album == nil {
				a.Album = ""
			} else {
				a.Album = *input.Album
			}
		}
		if has("grouping") {
			if input.Grouping == nil {
				a.Grouping = ""
			} else {
				a.Grouping = *input.Grouping
			}
		}
		if has("details") {
			if input.Details == nil {
				a.Details = ""
			} else {
				a.Details = *input.Details
			}
		}
		if has("audience") {
			if input.Audience == nil {
				a.Audience = ""
			} else {
				a.Audience = *input.Audience
			}
		}
		if has("content_type") {
			if input.ContentType == nil {
				a.ContentType = ""
			} else {
				a.ContentType = *input.ContentType
			}
		}
		if has("rating100") {
			a.Rating = input.Rating100
		}
		if input.Organized != nil {
			a.Organized = *input.Organized
		}
		if input.ResumeTime != nil {
			a.ResumeTime = *input.ResumeTime
		}
		if input.PlayDuration != nil {
			a.PlayDuration = *input.PlayDuration
		}
		if has("author_ids") {
			ids, e := parseAudioIDs(input.AuthorIds)
			if e != nil {
				return e
			}
			a.AuthorIDs = models.NewRelatedIDs(ids)
		}
		if has("tag_ids") {
			ids, e := parseAudioIDs(input.TagIds)
			if e != nil {
				return e
			}
			a.TagIDs = models.NewRelatedIDs(ids)
		}
		if has("genres") {
			a.Genres = input.Genres
		}
		if has("descriptors") {
			a.Descriptors = input.Descriptors
		}
		if input.PrimaryFileID != nil {
			value, e := strconv.Atoi(*input.PrimaryFileID)
			if e != nil {
				return e
			}
			fileID := models.FileID(value)
			a.PrimaryFileID = &fileID
		}
		if err := r.repository.Audio.Update(ctx, a); err != nil {
			return err
		}
		if input.CustomFields != nil {
			if err := r.repository.Audio.SetCustomFields(ctx, id, *input.CustomFields); err != nil {
				return err
			}
		}
		if input.CoverImage != nil {
			data, e := utils.ProcessImageInput(ctx, *input.CoverImage)
			if e != nil {
				return e
			}
			if e = r.repository.Audio.UpdateCover(ctx, id, data); e != nil {
				return e
			}
		}
		ret, err = r.repository.Audio.Find(ctx, id)
		return err
	})
	return ret, err
}

func (r *mutationResolver) AudioDestroy(ctx context.Context, id string) (bool, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return false, err
	}
	err = r.withTxn(ctx, func(ctx context.Context) error { return r.repository.Audio.Destroy(ctx, idInt) })
	return err == nil, err
}

func (r *mutationResolver) AudioSaveActivity(ctx context.Context, id string, resumeTime *float64, playDuration *float64) (ret bool, err error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return false, err
	}
	err = r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Audio.SaveActivity(ctx, idInt, resumeTime, playDuration)
		return err
	})
	return ret, err
}

func (r *mutationResolver) AudioIncrementPlayCount(ctx context.Context, id string) (ret int, err error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return 0, err
	}
	err = r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Audio.IncrementPlayCount(ctx, idInt)
		return err
	})
	return ret, err
}
