package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
	"gopkg.in/guregu/null.v4"
	"gopkg.in/guregu/null.v4/zero"
)

const (
	audioTable            = "audios"
	audiosFilesTable      = "audios_files"
	audioIDColumn         = "audio_id"
	audiosAuthorsTable    = "groups_audios"
	audiosTagsTable       = "audios_tags"
	audioGenresTable      = "audio_genres"
	audioDescriptorsTable = "audio_descriptors"
	audioCoverBlobColumn  = "cover_blob"
)

type audioRow struct {
	ID           int           `db:"id" goqu:"skipinsert"`
	Title        zero.String   `db:"title"`
	Album        zero.String   `db:"album"`
	Grouping     zero.String   `db:"grouping"`
	Details      zero.String   `db:"details"`
	Audience     zero.String   `db:"audience"`
	ContentType  zero.String   `db:"content_type"`
	Rating       null.Int      `db:"rating"`
	Organized    bool          `db:"organized"`
	ResumeTime   float64       `db:"resume_time"`
	PlayDuration float64       `db:"play_duration"`
	PlayCount    int           `db:"play_count"`
	LastPlayedAt NullTimestamp `db:"last_played_at"`
	CreatedAt    Timestamp     `db:"created_at"`
	UpdatedAt    Timestamp     `db:"updated_at"`
	// CoverBlob is managed independently through UpdateCover. Audio updates are
	// built from a models.Audio value, which intentionally does not load the
	// cover blob; including this field in a normal UPDATE would therefore erase
	// an existing cover whenever any metadata field changes.
	CoverBlob zero.String `db:"cover_blob" goqu:"skipupdate"`
}

func (r *audioRow) fromAudio(a models.Audio) {
	r.ID = a.ID
	r.Title = zero.StringFrom(a.Title)
	r.Album = zero.StringFrom(a.Album)
	r.Grouping = zero.StringFrom(a.Grouping)
	r.Details = zero.StringFrom(a.Details)
	r.Audience = zero.StringFrom(a.Audience)
	r.ContentType = zero.StringFrom(a.ContentType)
	r.Rating = intFromPtr(a.Rating)
	r.Organized = a.Organized
	r.ResumeTime = a.ResumeTime
	r.PlayDuration = a.PlayDuration
	r.PlayCount = a.PlayCount
	if a.LastPlayedAt != nil {
		r.LastPlayedAt = NullTimestamp{Timestamp: *a.LastPlayedAt, Valid: true}
	}
	r.CreatedAt = Timestamp{Timestamp: a.CreatedAt}
	r.UpdatedAt = Timestamp{Timestamp: a.UpdatedAt}
}

type audioQueryRow struct {
	audioRow
	PrimaryFileID       null.Int    `db:"primary_file_id"`
	PrimaryFolderPath   zero.String `db:"primary_folder_path"`
	PrimaryFileBasename zero.String `db:"primary_file_basename"`
}

func (r audioQueryRow) resolve() *models.Audio {
	a := &models.Audio{
		ID:            r.ID,
		Title:         r.Title.String,
		Album:         r.Album.String,
		Grouping:      r.Grouping.String,
		Details:       r.Details.String,
		Audience:      r.Audience.String,
		ContentType:   r.ContentType.String,
		Rating:        nullIntPtr(r.Rating),
		Organized:     r.Organized,
		ResumeTime:    r.ResumeTime,
		PlayDuration:  r.PlayDuration,
		PlayCount:     r.PlayCount,
		PrimaryFileID: nullIntFileIDPtr(r.PrimaryFileID),
		CreatedAt:     r.CreatedAt.Timestamp,
		UpdatedAt:     r.UpdatedAt.Timestamp,
	}
	if r.LastPlayedAt.Valid {
		t := r.LastPlayedAt.Timestamp
		a.LastPlayedAt = &t
	}
	if r.PrimaryFolderPath.Valid && r.PrimaryFileBasename.Valid {
		a.Path = filepath.Join(r.PrimaryFolderPath.String, r.PrimaryFileBasename.String)
	}
	return a
}

type AudioStore struct {
	blobJoinQueryBuilder
	customFieldsStore
	tableMgr *table
	repo     *storeRepository
}

func NewAudioStore(repo *storeRepository, blobStore *BlobStore) *AudioStore {
	return &AudioStore{
		blobJoinQueryBuilder: blobJoinQueryBuilder{blobStore: blobStore, joinTable: audioTable},
		customFieldsStore: customFieldsStore{
			table: audiosCustomFieldsTable,
			fk:    audiosCustomFieldsTable.Col(audioIDColumn),
		},
		tableMgr: audioTableMgr,
		repo:     repo,
	}
}

func (s *AudioStore) table() exp.IdentifierExpression { return s.tableMgr.table }

func (s *AudioStore) selectDataset() *goqu.SelectDataset {
	t := s.table()
	files := fileTableMgr.table
	folders := folderTableMgr.table
	return dialect.From(t).
		LeftJoin(audiosFilesJoinTable, goqu.On(
			audiosFilesJoinTable.Col(audioIDColumn).Eq(t.Col(idColumn)),
			audiosFilesJoinTable.Col("primary").Eq(1),
		)).
		LeftJoin(files, goqu.On(files.Col(idColumn).Eq(audiosFilesJoinTable.Col(fileIDColumn)))).
		LeftJoin(folders, goqu.On(folders.Col(idColumn).Eq(files.Col("parent_folder_id")))).
		Select(t.All(),
			audiosFilesJoinTable.Col(fileIDColumn).As("primary_file_id"),
			folders.Col("path").As("primary_folder_path"),
			files.Col("basename").As("primary_file_basename"),
		)
}

func (s *AudioStore) findOne(ctx context.Context, q *goqu.SelectDataset) (*models.Audio, error) {
	var row audioQueryRow
	if err := queryFunc(ctx, q, true, func(rows *sqlx.Rows) error { return rows.StructScan(&row) }); err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return row.resolve(), nil
}

func selectAudioValues[T any](ctx context.Context, q *goqu.SelectDataset, out *[]T) error {
	return queryFunc(ctx, q, false, func(rows *sqlx.Rows) error {
		var value T
		if err := rows.Scan(&value); err != nil {
			return err
		}
		*out = append(*out, value)
		return nil
	})
}

func (s *AudioStore) Find(ctx context.Context, id int) (*models.Audio, error) {
	return s.findOne(ctx, s.selectDataset().Where(s.tableMgr.byID(id)))
}

func (s *AudioStore) FindMany(ctx context.Context, ids []int) ([]*models.Audio, error) {
	ret := make([]*models.Audio, 0, len(ids))
	for _, id := range ids {
		a, err := s.Find(ctx, id)
		if err != nil {
			return nil, err
		}
		if a != nil {
			ret = append(ret, a)
		}
	}
	return ret, nil
}

func (s *AudioStore) FindByFileID(ctx context.Context, fileID models.FileID) ([]*models.Audio, error) {
	var ids []int
	q := dialect.From(audiosFilesJoinTable).Select(audioIDColumn).Where(goqu.Ex{fileIDColumn: fileID})
	if err := selectAudioValues(ctx, q, &ids); err != nil {
		return nil, err
	}
	return s.FindMany(ctx, ids)
}

func (s *AudioStore) CountByFileID(ctx context.Context, fileID models.FileID) (int, error) {
	q := dialect.From(audiosFilesJoinTable).Select(goqu.COUNT("*")).Where(goqu.Ex{fileIDColumn: fileID})
	return count(ctx, q)
}

func (s *AudioStore) Create(ctx context.Context, a *models.Audio, fileIDs []models.FileID) error {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = a.CreatedAt
	}
	var row audioRow
	row.fromAudio(*a)
	id, err := s.tableMgr.insertID(ctx, row)
	if err != nil {
		return err
	}
	a.ID = id
	if len(fileIDs) > 0 {
		if err := audiosFilesTableMgr.insertJoins(ctx, id, true, fileIDs); err != nil {
			return err
		}
	}
	if err := s.replaceRelationships(ctx, a); err != nil {
		return err
	}
	if a.CustomFields != nil {
		return s.SetCustomFields(ctx, id, models.CustomFieldsInput{Full: a.CustomFields})
	}
	return nil
}

func (s *AudioStore) Update(ctx context.Context, a *models.Audio) error {
	a.UpdatedAt = time.Now()
	var row audioRow
	row.fromAudio(*a)
	if err := s.tableMgr.updateByID(ctx, a.ID, row); err != nil {
		return err
	}
	if a.PrimaryFileID != nil {
		if _, err := exec(ctx, dialect.Update(audiosFilesJoinTable).Set(goqu.Record{"primary": false}).Where(goqu.Ex{audioIDColumn: a.ID})); err != nil {
			return err
		}
		if _, err := exec(ctx, dialect.Update(audiosFilesJoinTable).Set(goqu.Record{"primary": true}).Where(goqu.Ex{audioIDColumn: a.ID, fileIDColumn: *a.PrimaryFileID})); err != nil {
			return err
		}
	}
	return s.replaceRelationships(ctx, a)
}

func (s *AudioStore) replaceRelationships(ctx context.Context, a *models.Audio) error {
	if a.AuthorIDs.Loaded() {
		if err := replaceIntValues(ctx, audiosAuthorsTable, audioIDColumn, "group_id", a.ID, a.AuthorIDs.List(), nil); err != nil {
			return err
		}
	}
	if a.TagIDs.Loaded() {
		if err := replaceIntValues(ctx, audiosTagsTable, audioIDColumn, "tag_id", a.ID, a.TagIDs.List(), goqu.Record{"source": "manual"}); err != nil {
			return err
		}
	}
	if a.Genres != nil {
		if err := replaceNamedValues(ctx, audioGenresTable, a.ID, a.Genres); err != nil {
			return err
		}
	}
	if a.Descriptors != nil {
		if err := replaceNamedValues(ctx, audioDescriptorsTable, a.ID, a.Descriptors); err != nil {
			return err
		}
	}
	return nil
}

func replaceIntValues(ctx context.Context, tableName, ownerColumn, valueColumn string, ownerID int, values []int, extra goqu.Record) error {
	if _, err := exec(ctx, dialect.Delete(tableName).Where(goqu.Ex{ownerColumn: ownerID})); err != nil {
		return err
	}
	for _, value := range values {
		record := goqu.Record{ownerColumn: ownerID, valueColumn: value}
		for k, v := range extra {
			record[k] = v
		}
		if _, err := exec(ctx, dialect.Insert(tableName).Rows(record)); err != nil {
			return err
		}
	}
	return nil
}

func normalizeAudioValue(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func replaceNamedValues(ctx context.Context, tableName string, audioID int, values []string) error {
	if _, err := exec(ctx, dialect.Delete(tableName).Where(goqu.Ex{audioIDColumn: audioID})); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		normalized := normalizeAudioValue(value)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		if _, err := exec(ctx, dialect.Insert(tableName).Rows(goqu.Record{
			audioIDColumn: audioID, "value": value, "normalized_value": normalized,
		})); err != nil {
			return err
		}
	}
	return nil
}

func (s *AudioStore) Destroy(ctx context.Context, id int) error {
	return s.tableMgr.destroyExisting(ctx, []int{id})
}

func (s *AudioStore) GetFiles(ctx context.Context, id int) ([]*models.AudioFile, error) {
	q := dialect.From(audiosFilesJoinTable).Select(fileIDColumn).Where(goqu.Ex{audioIDColumn: id}).Order(goqu.I("primary").Desc())
	var ids []models.FileID
	if err := selectAudioValues(ctx, q, &ids); err != nil {
		return nil, err
	}
	files, err := s.repo.File.Find(ctx, ids...)
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AudioFile, 0, len(files))
	for _, f := range files {
		af, ok := f.(*models.AudioFile)
		if !ok {
			return nil, fmt.Errorf("audio %d references non-audio file %T", id, f)
		}
		ret = append(ret, af)
	}
	return ret, nil
}

func (s *AudioStore) getIntValues(ctx context.Context, tableName, valueColumn string, audioID int) ([]int, error) {
	var ret []int
	q := dialect.From(tableName).Select(valueColumn).Where(goqu.Ex{audioIDColumn: audioID}).Order(goqu.I(valueColumn).Asc())
	query, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}
	if err := dbWrapper.Select(ctx, &ret, query, args...); err != nil {
		return nil, err
	}
	return ret, nil
}

func (s *AudioStore) GetAuthorIDs(ctx context.Context, id int) ([]int, error) {
	return s.getIntValues(ctx, audiosAuthorsTable, "group_id", id)
}
func (s *AudioStore) GetTagIDs(ctx context.Context, id int) ([]int, error) {
	return s.getIntValues(ctx, audiosTagsTable, "tag_id", id)
}
func (s *AudioStore) getNamedValues(ctx context.Context, tableName string, id int) ([]string, error) {
	var ret []string
	q := dialect.From(tableName).Select("value").Where(goqu.Ex{audioIDColumn: id}).Order(goqu.I("value").Asc())
	query, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}
	if err := dbWrapper.Select(ctx, &ret, query, args...); err != nil {
		return nil, err
	}
	return ret, nil
}
func (s *AudioStore) GetGenres(ctx context.Context, id int) ([]string, error) {
	return s.getNamedValues(ctx, audioGenresTable, id)
}
func (s *AudioStore) GetDescriptors(ctx context.Context, id int) ([]string, error) {
	return s.getNamedValues(ctx, audioDescriptorsTable, id)
}

func (s *AudioStore) SaveActivity(ctx context.Context, id int, resumeTime, playDuration *float64) (bool, error) {
	record := goqu.Record{"updated_at": time.Now()}
	if resumeTime != nil {
		record["resume_time"] = *resumeTime
	}
	if playDuration != nil {
		record["play_duration"] = goqu.L("play_duration + ?", *playDuration)
	}
	if err := s.tableMgr.updateByID(ctx, id, record); err != nil {
		return false, err
	}
	return true, nil
}

func (s *AudioStore) IncrementPlayCount(ctx context.Context, id int) (int, error) {
	now := time.Now()
	if err := s.tableMgr.updateByID(ctx, id, goqu.Record{"play_count": goqu.L("play_count + 1"), "last_played_at": now}); err != nil {
		return 0, err
	}
	var countValue int
	q := dialect.From(audioTable).Select("play_count").Where(goqu.Ex{idColumn: id})
	if err := queryFunc(ctx, q, true, func(rows *sqlx.Rows) error { return rows.Scan(&countValue) }); err != nil {
		return 0, err
	}
	return countValue, nil
}

func (s *AudioStore) GetCover(ctx context.Context, id int) ([]byte, error) {
	return s.GetImage(ctx, id, audioCoverBlobColumn)
}
func (s *AudioStore) HasCover(ctx context.Context, id int) (bool, error) {
	return s.HasImage(ctx, id, audioCoverBlobColumn)
}
func (s *AudioStore) UpdateCover(ctx context.Context, id int, cover []byte) error {
	return s.UpdateImage(ctx, id, audioCoverBlobColumn, cover)
}

func audioStringCondition(column exp.IdentifierExpression, c *models.StringCriterionInput) exp.Expression {
	if c == nil {
		return nil
	}
	switch c.Modifier {
	case models.CriterionModifierEquals:
		return column.Eq(c.Value)
	case models.CriterionModifierNotEquals:
		return column.Neq(c.Value)
	case models.CriterionModifierExcludes:
		return column.NotLike("%" + c.Value + "%")
	default:
		return column.Like("%" + c.Value + "%")
	}
}

func (s *AudioStore) Query(ctx context.Context, options models.AudioQueryOptions) (*models.AudioQueryResult, error) {
	result := models.NewAudioQueryResult(s)
	t := s.table()
	q := dialect.From(t).Select(goqu.DISTINCT(t.Col(idColumn)))
	f := options.AudioFilter
	if f != nil {
		for col, criterion := range map[string]*models.StringCriterionInput{
			"title": f.Title, "album": f.Album, "grouping": f.Grouping,
			"audience": f.Audience, "content_type": f.ContentType,
		} {
			if ex := audioStringCondition(t.Col(col), criterion); ex != nil {
				q = q.Where(ex)
			}
		}
		if f.Organized != nil {
			q = q.Where(t.Col("organized").Eq(*f.Organized))
		}
		if f.Path != nil {
			files := fileTableMgr.table
			folders := folderTableMgr.table
			q = q.Join(audiosFilesJoinTable, goqu.On(audiosFilesJoinTable.Col(audioIDColumn).Eq(t.Col(idColumn)))).
				Join(files, goqu.On(files.Col(idColumn).Eq(audiosFilesJoinTable.Col(fileIDColumn)))).
				Join(folders, goqu.On(folders.Col(idColumn).Eq(files.Col("parent_folder_id")))).
				Where(goqu.L("(? || ? || ?) LIKE ?", folders.Col("path"), string(filepath.Separator), files.Col("basename"), "%"+f.Path.Value+"%"))
		}
		if f.Genre != nil {
			g := goqu.T(audioGenresTable)
			q = q.Join(g, goqu.On(g.Col(audioIDColumn).Eq(t.Col(idColumn)))).Where(audioStringCondition(g.Col("value"), f.Genre))
		}
		if f.Descriptor != nil {
			d := goqu.T(audioDescriptorsTable)
			q = q.Join(d, goqu.On(d.Col(audioIDColumn).Eq(t.Col(idColumn)))).Where(audioStringCondition(d.Col("value"), f.Descriptor))
		}
		if f.Authors != nil && len(f.Authors.Value) > 0 {
			ids := make([]int, 0, len(f.Authors.Value))
			for _, value := range f.Authors.Value {
				id, err := strconv.Atoi(value)
				if err != nil {
					return nil, err
				}
				ids = append(ids, id)
			}
			ga := goqu.T(audiosAuthorsTable)
			q = q.Join(ga, goqu.On(ga.Col(audioIDColumn).Eq(t.Col(idColumn)))).Where(ga.Col("group_id").In(ids))
		}
		if f.Rating100 != nil {
			clause, args := getIntCriterionWhereClause("audios.rating", *f.Rating100)
			q = q.Where(goqu.L(clause, args...))
		}
		if f.Duration != nil {
			durationExpr := "(SELECT audio_files.duration FROM audios_files JOIN audio_files ON audio_files.file_id = audios_files.file_id WHERE audios_files.audio_id = audios.id AND audios_files.\"primary\" = 1 LIMIT 1)"
			clause, args := getIntCriterionWhereClause("cast("+durationExpr+" as int)", *f.Duration)
			q = q.Where(goqu.L(clause, args...))
		}
	}
	find := options.FindFilter
	if find != nil && find.Q != nil && strings.TrimSpace(*find.Q) != "" {
		term := "%" + strings.TrimSpace(*find.Q) + "%"
		q = q.Where(goqu.Or(
			t.Col("title").Like(term), t.Col("album").Like(term), t.Col("details").Like(term),
			t.Col("audience").Like(term), t.Col("content_type").Like(term),
			goqu.L("EXISTS (SELECT 1 FROM groups_audios JOIN groups ON groups.id = groups_audios.group_id WHERE groups_audios.audio_id = audios.id AND (groups.name LIKE ? OR groups.aliases LIKE ?))", term, term),
			goqu.L("EXISTS (SELECT 1 FROM audio_genres WHERE audio_genres.audio_id = audios.id AND audio_genres.value LIKE ?)", term),
			goqu.L("EXISTS (SELECT 1 FROM audio_descriptors WHERE audio_descriptors.audio_id = audios.id AND audio_descriptors.value LIKE ?)", term),
		))
	}
	countQ := q.Select(goqu.COUNT(goqu.DISTINCT(t.Col(idColumn))))
	var err error
	result.Count, err = count(ctx, countQ)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	matchingQ := q
	sortCol := "title"
	direction := "ASC"
	if find != nil {
		allowed := map[string]bool{
			"title": true, "album": true, "audience": true, "rating": true,
			"created_at": true, "updated_at": true, "last_played_at": true, "play_count": true,
			"duration": true, "path": true, "author": true, "genre": true, "descriptor": true,
		}
		if candidate := find.GetSort(sortCol); allowed[candidate] {
			sortCol = candidate
		}
		direction = find.GetDirection()
	}
	sortExpressions := map[string]string{
		"duration":   "(SELECT audio_files.duration FROM audios_files JOIN audio_files ON audio_files.file_id = audios_files.file_id WHERE audios_files.audio_id = audios.id AND audios_files.\"primary\" = 1 LIMIT 1)",
		"path":       "(SELECT folders.path || ? || files.basename FROM audios_files JOIN files ON files.id = audios_files.file_id JOIN folders ON folders.id = files.parent_folder_id WHERE audios_files.audio_id = audios.id AND audios_files.\"primary\" = 1 LIMIT 1)",
		"author":     "(SELECT MIN(groups.name) FROM groups_audios JOIN groups ON groups.id = groups_audios.group_id WHERE groups_audios.audio_id = audios.id)",
		"genre":      "(SELECT MIN(value) FROM audio_genres WHERE audio_genres.audio_id = audios.id)",
		"descriptor": "(SELECT MIN(value) FROM audio_descriptors WHERE audio_descriptors.audio_id = audios.id)",
	}
	var order exp.OrderedExpression
	if raw, ok := sortExpressions[sortCol]; ok {
		literal := goqu.L(raw, string(filepath.Separator))
		if sortCol != "path" {
			literal = goqu.L(raw)
		}
		if direction == "DESC" {
			order = literal.Desc()
		} else {
			order = literal.Asc()
		}
	} else if direction == "DESC" {
		order = t.Col(sortCol).Desc()
	} else {
		order = t.Col(sortCol).Asc()
	}
	q = q.Order(order, t.Col(idColumn).Asc())
	if find == nil || !find.IsGetAll() {
		page, size := 1, 25
		if find != nil {
			page, size = find.GetPage(), find.GetPageSize()
		}
		q = q.Limit(uint(size)).Offset(uint((page - 1) * size))
	}
	if err := selectAudioValues(ctx, q, &result.IDs); err != nil {
		return nil, err
	}
	stats := struct {
		Duration float64 `db:"duration"`
		Size     float64 `db:"size"`
	}{}
	statsQ := dialect.From(audioTable).
		Join(audiosFilesJoinTable, goqu.On(audiosFilesJoinTable.Col(audioIDColumn).Eq(goqu.T(audioTable).Col(idColumn)), audiosFilesJoinTable.Col("primary").Eq(1))).
		Join(audioFileTableMgr.table, goqu.On(audioFileTableMgr.table.Col(fileIDColumn).Eq(audiosFilesJoinTable.Col(fileIDColumn)))).
		Join(fileTableMgr.table, goqu.On(fileTableMgr.table.Col(idColumn).Eq(audiosFilesJoinTable.Col(fileIDColumn)))).
		Select(goqu.COALESCE(goqu.SUM(audioFileTableMgr.table.Col("duration")), 0).As("duration"), goqu.COALESCE(goqu.SUM(fileTableMgr.table.Col("size")), 0).As("size")).
		Where(goqu.T(audioTable).Col(idColumn).In(matchingQ))
	_ = queryFunc(ctx, statsQ, true, func(rows *sqlx.Rows) error { return rows.StructScan(&stats) })
	result.TotalDuration, result.TotalSize = stats.Duration, stats.Size
	return result, nil
}
