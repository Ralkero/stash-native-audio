package models

import "context"

type AudioFileLoader interface {
	GetFiles(ctx context.Context, relatedID int) ([]*AudioFile, error)
}

type AudioGetter interface {
	Find(ctx context.Context, id int) (*Audio, error)
	FindMany(ctx context.Context, ids []int) ([]*Audio, error)
}

type AudioReader interface {
	AudioGetter
	AudioFileLoader
	FindByFileID(ctx context.Context, fileID FileID) ([]*Audio, error)
	CountByFileID(ctx context.Context, fileID FileID) (int, error)
	Query(ctx context.Context, options AudioQueryOptions) (*AudioQueryResult, error)
	GetAuthorIDs(ctx context.Context, audioID int) ([]int, error)
	GetTagIDs(ctx context.Context, audioID int) ([]int, error)
	GetGenres(ctx context.Context, audioID int) ([]string, error)
	GetDescriptors(ctx context.Context, audioID int) ([]string, error)
	GetCustomFields(ctx context.Context, audioID int) (map[string]interface{}, error)
	GetCover(ctx context.Context, audioID int) ([]byte, error)
	HasCover(ctx context.Context, audioID int) (bool, error)
}

type AudioWriter interface {
	CustomFieldsWriter
	Create(ctx context.Context, audio *Audio, fileIDs []FileID) error
	Update(ctx context.Context, audio *Audio) error
	Destroy(ctx context.Context, id int) error
	SaveActivity(ctx context.Context, id int, resumeTime *float64, playDuration *float64) (bool, error)
	IncrementPlayCount(ctx context.Context, id int) (int, error)
	UpdateCover(ctx context.Context, audioID int, cover []byte) error
}

type AudioReaderWriter interface {
	AudioReader
	AudioWriter
}
