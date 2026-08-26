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

func TestExtractNote(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		botUserID string
		want      string
	}{
		{
			name:      "URL only",
			text:      "<@BOT123> <https://github.com/pyama86/jobpin/actions/runs/123>",
			botUserID: "BOT123",
			want:      "",
		},
		{
			name:      "URL, mention and text",
			text:      "<@BOT123> <https://github.com/pyama86/jobpin/actions/runs/123> <@U0HOGE> デプロイ完了しました",
			botUserID: "BOT123",
			want:      "<@U0HOGE> デプロイ完了しました",
		},
		{
			name:      "URL with label",
			text:      "<@BOT123> <https://github.com/pyama86/jobpin/actions/runs/123|CI run> よろしく",
			botUserID: "BOT123",
			want:      "よろしく",
		},
		{
			name:      "multiple URLs",
			text:      "<@BOT123> <https://github.com/a/b/actions/runs/1> と <https://github.com/c/d/actions/runs/2|run2> をお願いします",
			botUserID: "BOT123",
			want:      "と をお願いします",
		},
		{
			name:      "empty botUserID falls back to leading mention",
			text:      "<@BOT123> <https://github.com/pyama86/jobpin/actions/runs/123> デプロイ完了しました",
			botUserID: "",
			want:      "デプロイ完了しました",
		},
		{
			name:      "text before URL",
			text:      "<@BOT123> デプロイ完了 <https://github.com/pyama86/jobpin/actions/runs/123>",
			botUserID: "BOT123",
			want:      "デプロイ完了",
		},
		{
			name:      "multiline",
			text:      "<@BOT123> <https://github.com/pyama86/jobpin/actions/runs/123>\n<@U0HOGE>\nデプロイ完了しました\nよろしくお願いします",
			botUserID: "BOT123",
			want:      "<@U0HOGE>\nデプロイ完了しました\nよろしくお願いします",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNote(tt.text, tt.botUserID)
			if got != tt.want {
				t.Errorf("extractNote(%q, %q) = %q, want %q", tt.text, tt.botUserID, got, tt.want)
			}
		})
	}
}
