package internal

import "testing"

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
		{1099511627776, "1.0 TB"},
		{1649267441664, "1.5 TB"},
	}

	for _, tt := range tests {
		got := HumanSize(tt.bytes)
		if got != tt.want {
			t.Errorf("HumanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
