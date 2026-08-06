package bot

import (
	"reflect"
	"testing"
)

func TestParseRunURLs(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []RunRef
	}{
		{
			name: "plain URL",
			text: "https://github.com/pyama86/jobpin/actions/runs/123",
			want: []RunRef{{Owner: "pyama86", Repo: "jobpin", RunID: 123}},
		},
		{
			name: "slack angle brackets",
			text: "<@U123> watch <https://github.com/pyama86/jobpin/actions/runs/456>",
			want: []RunRef{{Owner: "pyama86", Repo: "jobpin", RunID: 456}},
		},
		{
			name: "slack URL with label",
			text: "<https://github.com/foo/bar/actions/runs/789|CI run>",
			want: []RunRef{{Owner: "foo", Repo: "bar", RunID: 789}},
		},
		{
			name: "multiple URLs",
			text: "check <https://github.com/a/b/actions/runs/1> and <https://github.com/c/d/actions/runs/2|run2>",
			want: []RunRef{
				{Owner: "a", Repo: "b", RunID: 1},
				{Owner: "c", Repo: "d", RunID: 2},
			},
		},
		{
			name: "mixed text",
			text: "デプロイの https://github.com/pyama86/jobpin/actions/runs/999 が終わったら教えて",
			want: []RunRef{{Owner: "pyama86", Repo: "jobpin", RunID: 999}},
		},
		{
			name: "no URL",
			text: "<@U123> hello",
			want: nil,
		},
		{
			name: "non-run github URL",
			text: "https://github.com/pyama86/jobpin/pull/1",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRunURLs(tt.text)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRunURLs(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}
