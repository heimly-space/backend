package httpdto

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDateUnmarshalValid(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"2024-02-29"`), &d); err != nil {
		t.Fatalf("unmarshal valid date: %v", err)
	}

	want := time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC)
	if !d.Time().Equal(want) {
		t.Fatalf("unexpected date: got %v want %v", d.Time(), want)
	}
}

func TestDateUnmarshalNullAndEmpty(t *testing.T) {
	t.Run("null", func(t *testing.T) {
		var d Date
		if err := json.Unmarshal([]byte(`null`), &d); err != nil {
			t.Fatalf("unmarshal null: %v", err)
		}
		if !d.IsZero() {
			t.Fatal("expected zero date for null")
		}
	})

	t.Run("empty-string", func(t *testing.T) {
		var d Date
		if err := json.Unmarshal([]byte(`""`), &d); err != nil {
			t.Fatalf("unmarshal empty: %v", err)
		}
		if !d.IsZero() {
			t.Fatal("expected zero date for empty string")
		}
	})
}

func TestDateUnmarshalInvalid(t *testing.T) {
	var d Date
	err := json.Unmarshal([]byte(`"29-02-2024"`), &d)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestDateMarshal(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		var d Date
		got, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal zero date: %v", err)
		}
		if string(got) != "null" {
			t.Fatalf("unexpected zero marshal value: %s", got)
		}
	})

	t.Run("non-zero", func(t *testing.T) {
		d := Date(time.Date(1995, time.October, 15, 0, 0, 0, 0, time.UTC))
		got, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal date: %v", err)
		}
		if string(got) != `"1995-10-15"` {
			t.Fatalf("unexpected marshal value: %s", got)
		}
	})
}
