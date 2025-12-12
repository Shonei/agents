package index

import (
	"reflect"
	"testing"
)

func TestExtractChunks(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "single chunk",
			text: "<chunk>content</chunk>",
			want: []string{"content"},
		},
		{
			name: "multiple chunks",
			text: "<chunk>chunk1</chunk><chunk>chunk2</chunk>",
			want: []string{"chunk1", "chunk2"},
		},
		{
			name: "chunks with newlines",
			text: "<chunk>\nchunk1\n</chunk>\n<chunk>chunk2</chunk>",
			want: []string{"chunk1", "chunk2"},
		},
		{
			name: "chunks with text outside",
			text: "ignore<chunk>chunk1</chunk>ignore<chunk>chunk2</chunk>ignore",
			want: []string{"chunk1", "chunk2"},
		},
		{
			name: "no chunks",
			text: "just text",
			want: []string{"just text"},
		},
		{
			name: "empty input",
			text: "",
			want: nil,
		},
		{
			name: "nested content",
			text: "<chunk>some code: <div>test</div></chunk>",
			want: []string{"some code: <div>test</div>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractChunks(tt.text)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractChunks() = %v, want %v", got, tt.want)
			}
		})
	}
}
