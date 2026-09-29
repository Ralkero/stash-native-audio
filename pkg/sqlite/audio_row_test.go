package sqlite

import (
	"strings"
	"testing"

	"gopkg.in/guregu/null.v4/zero"
)

func TestAudioRowSkipsCoverBlobOnUpdate(t *testing.T) {
	row := audioRow{
		ID:        1,
		Title:     zero.StringFrom("Updated title"),
		CoverBlob: zero.StringFrom("existing cover"),
	}

	sql, _, err := dialect.Update(audioTable).Set(row).ToSQL()
	if err != nil {
		t.Fatalf("building audio update SQL: %v", err)
	}
	if strings.Contains(sql, audioCoverBlobColumn) {
		t.Fatalf("ordinary audio update must not write %s: %s", audioCoverBlobColumn, sql)
	}
	if !strings.Contains(sql, "title") {
		t.Fatalf("ordinary audio update must still write metadata fields: %s", sql)
	}
}
