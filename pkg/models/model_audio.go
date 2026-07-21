package models

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"time"
)

// Audio stores metadata for a native audio item. Audio is intentionally a
// separate entity from Scene so video-only plugins remain type-safe.
type Audio struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Album       string `json:"album"`
	Grouping    string `json:"grouping"`
	Details     string `json:"details"`
	Audience    string `json:"audience"`
	ContentType string `json:"content_type"`
	Rating      *int   `json:"rating"`
	Organized   bool   `json:"organized"`

	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ResumeTime   float64    `json:"resume_time"`
	PlayDuration float64    `json:"play_duration"`
	PlayCount    int        `json:"play_count"`
	LastPlayedAt *time.Time `json:"last_played_at"`

	Files         RelatedAudioFiles
	PrimaryFileID *FileID
	Path          string
	AuthorIDs     RelatedIDs
	TagIDs        RelatedIDs
	Genres        []string
	Descriptors   []string
	CustomFields  CustomFieldMap
}

func NewAudio() Audio {
	now := time.Now()
	return Audio{CreatedAt: now, UpdatedAt: now}
}

func (a Audio) GetTitle() string {
	if a.Title != "" {
		return a.Title
	}
	return filepath.Base(a.Path)
}

func (a Audio) DisplayName() string {
	if a.Path != "" {
		return a.Path
	}
	return strconv.Itoa(a.ID)
}

func (a *Audio) LoadFiles(ctx context.Context, l AudioFileLoader) error {
	return a.Files.load(func() ([]*AudioFile, error) { return l.GetFiles(ctx, a.ID) })
}

func (a *Audio) LoadPrimaryFile(ctx context.Context, l FileGetter) error {
	return a.Files.loadPrimary(func() (*AudioFile, error) {
		if a.PrimaryFileID == nil {
			return nil, nil
		}
		files, err := l.Find(ctx, *a.PrimaryFileID)
		if err != nil || len(files) == 0 {
			return nil, err
		}
		f, ok := files[0].(*AudioFile)
		if !ok {
			return nil, errors.New("not an audio file")
		}
		return f, nil
	})
}

type RelatedAudioFiles struct {
	primaryFile   *AudioFile
	files         []*AudioFile
	primaryLoaded bool
}

func NewRelatedAudioFiles(files []*AudioFile) RelatedAudioFiles {
	return RelatedAudioFiles{files: files}
}

func (r *RelatedAudioFiles) Set(files []*AudioFile) { r.files = files }
func (r *RelatedAudioFiles) SetPrimary(f *AudioFile) {
	r.primaryFile = f
	r.primaryLoaded = true
}
func (r RelatedAudioFiles) Loaded() bool        { return r.files != nil }
func (r RelatedAudioFiles) List() []*AudioFile  { return r.files }
func (r RelatedAudioFiles) Primary() *AudioFile { return r.primaryFile }
func (r *RelatedAudioFiles) load(fn func() ([]*AudioFile, error)) error {
	if r.Loaded() {
		return nil
	}
	files, err := fn()
	if err != nil {
		return err
	}
	if files == nil {
		files = []*AudioFile{}
	}
	r.files = files
	return nil
}
func (r *RelatedAudioFiles) loadPrimary(fn func() (*AudioFile, error)) error {
	if r.primaryLoaded {
		return nil
	}
	f, err := fn()
	if err != nil {
		return err
	}
	r.primaryFile = f
	r.primaryLoaded = true
	return nil
}

type AudioFilterType struct {
	Title       *StringCriterionInput `json:"title"`
	Album       *StringCriterionInput `json:"album"`
	Grouping    *StringCriterionInput `json:"grouping"`
	Audience    *StringCriterionInput `json:"audience"`
	ContentType *StringCriterionInput `json:"content_type"`
	Path        *StringCriterionInput `json:"path"`
	Genre       *StringCriterionInput `json:"genre"`
	Descriptor  *StringCriterionInput `json:"descriptor"`
	Authors     *MultiCriterionInput  `json:"authors"`
	Rating100   *IntCriterionInput    `json:"rating100"`
	Organized   *bool                 `json:"organized"`
	Duration    *IntCriterionInput    `json:"duration"`
}

type AudioQueryOptions struct {
	QueryOptions
	AudioFilter   *AudioFilterType
	TotalDuration bool
	TotalSize     bool
}

type AudioQueryResult struct {
	QueryResult[int]
	TotalDuration float64
	TotalSize     float64
	getter        AudioGetter
	audios        []*Audio
	resolveErr    error
}

func NewAudioQueryResult(getter AudioGetter) *AudioQueryResult {
	return &AudioQueryResult{getter: getter}
}

func (r *AudioQueryResult) Resolve(ctx context.Context) ([]*Audio, error) {
	if r.audios == nil && r.resolveErr == nil {
		r.audios, r.resolveErr = r.getter.FindMany(ctx, r.IDs)
	}
	return r.audios, r.resolveErr
}

type AudioCreateInput struct {
	Title        *string        `json:"title"`
	Album        *string        `json:"album"`
	Grouping     *string        `json:"grouping"`
	Details      *string        `json:"details"`
	Audience     *string        `json:"audience"`
	ContentType  *string        `json:"content_type"`
	Rating100    *int           `json:"rating100"`
	Organized    *bool          `json:"organized"`
	AuthorIds    []string       `json:"author_ids"`
	TagIds       []string       `json:"tag_ids"`
	Genres       []string       `json:"genres"`
	Descriptors  []string       `json:"descriptors"`
	FileIds      []string       `json:"file_ids"`
	CoverImage   *string        `json:"cover_image"`
	CustomFields map[string]any `json:"custom_fields"`
}

type AudioUpdateInput struct {
	ClientMutationID *string            `json:"clientMutationId"`
	ID               string             `json:"id"`
	Title            *string            `json:"title"`
	Album            *string            `json:"album"`
	Grouping         *string            `json:"grouping"`
	Details          *string            `json:"details"`
	Audience         *string            `json:"audience"`
	ContentType      *string            `json:"content_type"`
	Rating100        *int               `json:"rating100"`
	Organized        *bool              `json:"organized"`
	AuthorIds        []string           `json:"author_ids"`
	TagIds           []string           `json:"tag_ids"`
	Genres           []string           `json:"genres"`
	Descriptors      []string           `json:"descriptors"`
	PrimaryFileID    *string            `json:"primary_file_id"`
	ResumeTime       *float64           `json:"resume_time"`
	PlayDuration     *float64           `json:"play_duration"`
	CoverImage       *string            `json:"cover_image"`
	CustomFields     *CustomFieldsInput `json:"custom_fields"`
}
