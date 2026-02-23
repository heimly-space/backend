package httpdto

import (
	"fmt"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

type Date time.Time

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*d = Date(time.Time{})
		return nil
	}

	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("invalid date format, expected YYYY-MM-DD")
	}

	*d = Date(t)
	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte(`null`), nil
	}
	return []byte(`"` + time.Time(d).Format(dateLayout) + `"`), nil
}

func (d Date) IsZero() bool {
	return time.Time(d).IsZero()
}

func (d Date) Time() time.Time {
	return time.Time(d)
}
