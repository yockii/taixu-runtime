package socialnet

import "testing"

// TestCommentTargetKey 验 C6：去重对象键——优先按被回复评论，其次按帖，取不到则空（放行）。
func TestCommentTargetKey(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"回复某评论", map[string]any{"post_id": "p1", "parent_comment_id": "c9"}, "reply:c9"},
		{"顶层评论某帖", map[string]any{"post_id": "p1"}, "postcomment:p1"},
		{"parent优先于post", map[string]any{"post_id": "p1", "parent_comment_id": "c9", "body": "hi"}, "reply:c9"},
		{"空parent退回post", map[string]any{"post_id": "p1", "parent_comment_id": ""}, "postcomment:p1"},
		{"都没有→空放行", map[string]any{"body": "hi"}, ""},
		{"空map→空", map[string]any{}, ""},
	}
	for _, c := range cases {
		if got := commentTargetKey(c.args); got != c.want {
			t.Errorf("%s: commentTargetKey=%q, 期 %q", c.name, got, c.want)
		}
	}
}
