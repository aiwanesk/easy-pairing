package mdb

import (
	"testing"
	"time"
)

func TestPeople(t *testing.T) {
	db, err := Open("testdata/people.mdb")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tb, err := db.Table("people")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := db.Scan(tb, func(r map[string]any) error { rows = append(rows, r); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "Ada" || rows[0]["nickname"] != "Countess" || rows[0]["active"] != true {
		t.Errorf("row 0: %v", rows[0])
	}
	if rows[1]["nickname"] != nil || rows[1]["active"] != false {
		t.Errorf("row 1: %v", rows[1])
	}
	if d, _ := rows[0]["created_at"].(time.Time); !d.Equal(time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date: %v", rows[0]["created_at"])
	}
}

func TestUnicodeAndNulls(t *testing.T) {
	db, err := Open("testdata/chinese.mdb")
	if err != nil {
		t.Fatal(err)
	}
	tb, _ := db.Table("chinese")
	var names []string
	db.Scan(tb, func(r map[string]any) error { names = append(names, r["name"].(string)); return nil })
	if len(names) != 4 || names[0] != "张三" || names[3] != "混合Mixed中English文" {
		t.Errorf("names: %q", names)
	}

	db2, err := Open("testdata/nulltest.mdb")
	if err != nil {
		t.Fatal(err)
	}
	tb2, _ := db2.Table("nulltest")
	var rows []map[string]any
	db2.Scan(tb2, func(r map[string]any) error { rows = append(rows, r); return nil })
	if rows[0]["val_int"] != nil || rows[1]["val_int"] != int64(42) || rows[1]["val_text"] != "not null" {
		t.Errorf("nulls: %v", rows)
	}
}
