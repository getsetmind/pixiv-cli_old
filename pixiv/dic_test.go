package pixiv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// dicSearchPage is a trimmed copy of a real /search result page, including the
// sidebar cards that must not be mistaken for hits.
const dicSearchPage = `<!DOCTYPE html>
<html lang="ja">
<head><title>初音ミク の検索結果(1ページ目)</title></head>
<body>
<div id="content">
  <div id="main" class="search" data-target="初音ミク">
    <header id="search-title">
      <h1>初音ミク（1ページ目）</h1>
      <div class="info">検索結果：4577件</div>
    </header>
    <section>
      <article>
        <div class="thumb">
          <a href="/a/%E5%88%9D%E9%9F%B3%E3%83%9F%E3%82%AF">
            <img src="https://i.pximg.net/c/128x128/img-master/miku.jpg" alt="" />
          </a>
        </div>
        <div class="info">
          <h2><a href="/a/%E5%88%9D%E9%9F%B3%E3%83%9F%E3%82%AF">初音ミク</a></h2>
          <p class="summary">
            クリプトン社のバーチャルシンガー <a href="/a/%E5%88%9D%E9%9F%B3%E3%83%9F%E3%82%AF">続きを読む</a>
          </p>
          <ul class="data">
            <li>更新: 2026-09-04 16:29:15</li>
            <li>閲覧数: 2,052,836</li>
            <li>作品数: 750903</li>
            <li>チェックリスト数: 609</li>
          </ul>
          <div class="relation">
            <span>関連</span>
            <ul>
              <li><a href="/a/KAITO">KAITO</a></li>
              <li><a href="/a/MEIKO">MEIKO</a></li>
            </ul>
          </div>
        </div>
      </article>
      <article>
        <div class="thumb"><a href="/a/%E6%A1%9C%E3%83%9F%E3%82%AF"><img src="https://i.pximg.net/c/128x128/img-master/sakura.jpg" alt="" /></a></div>
        <div class="info">
          <h2><a href="/a/%E6%A1%9C%E3%83%9F%E3%82%AF">桜ミク</a></h2>
          <p class="summary">春季仕様のミク。</p>
          <ul class="data">
            <li>更新: 2026-09-23 07:47:12</li>
            <li>閲覧数: 287088</li>
            <li>作品数: 11392</li>
            <li>チェックリスト数: 87</li>
          </ul>
          <div class="relation">
            <span>関連</span>
            <ul><li><a href="/a/%E9%9B%AA%E3%83%9F%E3%82%AF">雪ミク</a></li></ul>
          </div>
        </div>
      </article>
    </section>
  </div>
  <aside id="sub-contents">
    <section class="spotlight">
      <article><div class="info"><h2><a href="/a/SPOTLIGHT">サイドバーの記事</a></h2></div></article>
    </section>
  </aside>
</div>
</body>
</html>`

// dicEmptySearchPage is what the site returns for a query that matched nothing:
// HTTP 404 carrying a valid page with no cards.
const dicEmptySearchPage = `<html><body>
<div id="content">
  <div id="main" class="search" data-target="zzz">
    <header id="search-title"><h1>zzz（1ページ目）</h1><div class="info">検索結果：0件</div></header>
    <section></section>
  </div>
</div>
</body></html>`

func newDicTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	cfg := DefaultConfig()
	cfg.DicBaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 0
	return NewClient(cfg)
}

func TestDicSearch(t *testing.T) {
	var gotPath, gotQuery string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(dicSearchPage))
	})
	c := newDicTestClient(t, h)

	arts, err := c.SearchArticles(context.Background(), "初音ミク", 2)
	if err != nil {
		t.Fatalf("SearchArticles: %v", err)
	}
	if gotPath != "/search" {
		t.Errorf("path = %q, want /search", gotPath)
	}
	if gotQuery != "初音ミク" {
		t.Errorf("query = %q, want 初音ミク", gotQuery)
	}
	if len(arts) != 2 {
		t.Fatalf("got %d articles, want 2 (the sidebar card must be ignored)", len(arts))
	}

	first := arts[0]
	if first.Title != "初音ミク" {
		t.Errorf("title = %q, want 初音ミク", first.Title)
	}
	if want := "https://dic.pixiv.net/a/%E5%88%9D%E9%9F%B3%E3%83%9F%E3%82%AF"; first.URL != want {
		t.Errorf("url = %q, want %q", first.URL, want)
	}
	if want := "クリプトン社のバーチャルシンガー"; first.Summary != want {
		t.Errorf("summary = %q, want %q", first.Summary, want)
	}
	if first.Updated != "2026-09-04 16:29:15" {
		t.Errorf("updated = %q", first.Updated)
	}
	if first.Views != 2052836 {
		t.Errorf("views = %d, want 2052836 (thousands separators stripped)", first.Views)
	}
	if first.Works != 750903 {
		t.Errorf("works = %d, want 750903", first.Works)
	}
	if first.Checklists != 609 {
		t.Errorf("checklists = %d, want 609", first.Checklists)
	}
	if want := "KAITO, MEIKO"; first.Related != want {
		t.Errorf("related = %q, want %q", first.Related, want)
	}
	if first.Thumbnail == "" {
		t.Error("thumbnail is empty")
	}

	if second := arts[1]; second.Title != "桜ミク" || second.Related != "雪ミク" {
		t.Errorf("second article = %+v", second)
	}
}

func TestDicSearchWithoutRelatedLinks(t *testing.T) {
	page := `<div id="main" class="search"><article>
		<div class="info"><h2><a href="/a/x">X</a></h2></div>
	</article></div>`
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(page)) })
	c := newDicTestClient(t, h)

	arts, err := c.SearchArticles(context.Background(), "x", 1)
	if err != nil {
		t.Fatalf("SearchArticles: %v", err)
	}
	if len(arts) != 1 {
		t.Fatalf("got %d articles, want 1", len(arts))
	}
	if arts[0].Related != "" || arts[0].Views != 0 || arts[0].Summary != "" {
		t.Errorf("optional fields should stay empty: %+v", arts[0])
	}
}

func TestDicSearchEmpty(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(dicEmptySearchPage))
	})
	c := newDicTestClient(t, h)

	arts, err := c.SearchArticles(context.Background(), "zzz", 1)
	if err != nil {
		t.Fatalf("SearchArticles: %v", err)
	}
	if len(arts) != 0 {
		t.Errorf("got %d articles, want 0", len(arts))
	}
}

func TestDicSearchServerError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	c := newDicTestClient(t, h)

	_, err := c.SearchArticles(context.Background(), "x", 1)
	if err == nil {
		t.Fatal("expected an error for a 5xx response")
	}
	if !isHTTPStatus(err, http.StatusInternalServerError) {
		t.Errorf("err = %v, want an HTTPError with status 500", err)
	}
}

func TestDicSearchUnparseable(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html><body><p>no results container</p></body></html>"))
	})
	c := newDicTestClient(t, h)

	if _, err := c.SearchArticles(context.Background(), "x", 1); err == nil {
		t.Fatal("expected an error when the page has no result container")
	}
}

func TestDicArticle(t *testing.T) {
	nodes := `[{"tag":"header","children":[{"tag":"text","text":"概要"}]},{"tag":"p","children":[{"tag":"italic","children":[{"tag":"text","text":" 引用"}]},{"tag":"text","text":"本文。"},{"tag":"article_link","text":"初音ミク","to":"初音ミク"}]}]`

	mux := http.NewServeMux()
	mux.HandleFunc("/_api/get_article/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("lang"); got != "ja" {
			t.Errorf("lang = %q, want ja", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":                       73,
			"tagName":                  "初音ミク",
			"translatedTagName":        "Hatsune Miku",
			"yomigana":                 "はつねみく",
			"abstract":                 "電子の歌姫。",
			"categories":               []string{"キャラクター", "音楽"},
			"nodes":                    nodes,
			"tagNameListInArticleJa":   []string{"3dcg", "ネギ"},
			"recommendedArticles":      []map[string]any{{"tagName": "GUMI"}, {"tagName": "KAITO"}},
			"mainIllust":               map[string]any{"imageUrl": "https://i.pximg.net/c/260x260/miku.jpg"},
			"updatedAtTimestamp":       1788506955,
			"relatedArticles":          map[string]any{"parent_article": nil},
			"illustListInArticle":      []any{},
			"novelListInArticle":       []any{},
			"relatedPixivisionArticle": nil,
		})
	})
	mux.HandleFunc("/_api/get_article_info/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"articleViewCount": 2052836,
			"commentCount":     15,
			"pixivWorkCount":   750903,
			"checklistCount":   609,
			"lastUpdatedAt":    21,
		})
	})
	c := newDicTestClient(t, mux)

	e, err := c.Article(context.Background(), "初音ミク", "ja")
	if err != nil {
		t.Fatalf("Article: %v", err)
	}
	if e.ID != 73 {
		t.Errorf("id = %d, want 73", e.ID)
	}
	if e.Title != "初音ミク" {
		t.Errorf("title = %q", e.Title)
	}
	if e.Yomigana != "はつねみく" {
		t.Errorf("yomigana = %q", e.Yomigana)
	}
	if e.Translation != "Hatsune Miku" {
		t.Errorf("translation = %q", e.Translation)
	}
	if e.Categories != "キャラクター, 音楽" {
		t.Errorf("categories = %q", e.Categories)
	}
	if e.Related != "GUMI, KAITO" {
		t.Errorf("related = %q", e.Related)
	}
	if len(e.Tags) != 2 || e.Tags[0] != "3dcg" {
		t.Errorf("tags = %v", e.Tags)
	}
	if e.Views != 2052836 || e.Comments != 15 || e.Works != 750903 || e.Checklists != 609 {
		t.Errorf("counters = %d/%d/%d/%d", e.Views, e.Comments, e.Works, e.Checklists)
	}
	if want := "2026-09-04 16:29:15"; e.Updated != want {
		t.Errorf("updated = %q, want %q", e.Updated, want)
	}
	if want := "https://dic.pixiv.net/a/%E5%88%9D%E9%9F%B3%E3%83%9F%E3%82%AF"; e.URL != want {
		t.Errorf("url = %q, want %q", e.URL, want)
	}
	if e.Thumbnail != "https://i.pximg.net/c/260x260/miku.jpg" {
		t.Errorf("thumbnail = %q", e.Thumbnail)
	}
	if !strings.Contains(e.Body, "## 概要") {
		t.Errorf("body is missing the heading: %q", e.Body)
	}
	if !strings.Contains(e.Body, "本文。") {
		t.Errorf("body is missing the paragraph: %q", e.Body)
	}
	if strings.Contains(e.Body, "続きを読む") {
		t.Errorf("body kept a read-more link: %q", e.Body)
	}
}

func TestDicArticleEnglish(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_api/get_article/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 20, "tagName": "Hatsune Miku"})
	})
	mux.HandleFunc("/_api/get_article_info/", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	c := newDicTestClient(t, mux)

	e, err := c.Article(context.Background(), "Hatsune Miku", "en")
	if err != nil {
		t.Fatalf("Article: %v", err)
	}
	if want := "https://dic.pixiv.net/en/a/Hatsune%20Miku"; e.URL != want {
		t.Errorf("url = %q, want %q", e.URL, want)
	}
}

func TestDicArticleNotFound(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"tagName": "missing"})
	})
	c := newDicTestClient(t, h)

	_, err := c.Article(context.Background(), "missing", "ja")
	if err == nil {
		t.Fatal("expected an error for a missing article")
	}
	if !isHTTPStatus(err, http.StatusNotFound) {
		t.Errorf("err = %v, want an HTTPError with status 404", err)
	}
}

func TestDicArticleSurvivesInfoFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_api/get_article/", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "tagName": "T", "categories": []string{"a"}})
	})
	mux.HandleFunc("/_api/get_article_info/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	c := newDicTestClient(t, mux)

	e, err := c.Article(context.Background(), "T", "ja")
	if err != nil {
		t.Fatalf("Article: %v", err)
	}
	if e.Title != "T" || e.Views != 0 {
		t.Errorf("entry = %+v", e)
	}
}

func TestDicNodesText(t *testing.T) {
	raw := `[{"tag":"header","children":[{"tag":"text","text":"概要"}]},{"tag":"p","children":[{"tag":"text","text":"本文。"},{"tag":"br"},{"tag":"text","text":"次の行"}]},{"tag":"unordered_list","children":[{"tag":"list_item","children":[{"tag":"bold","children":[{"tag":"text","text":"太字"}]}]}]},{"tag":"pixiv_image","id":123}]`
	want := "## 概要\n\n本文。\n次の行\n\n- 太字"
	if got := dicNodesText(raw); got != want {
		t.Errorf("dicNodesText() = %q, want %q", got, want)
	}
	for _, bad := range []string{"", "not json", "{}"} {
		if got := dicNodesText(bad); got != "" {
			t.Errorf("dicNodesText(%q) = %q, want empty", bad, got)
		}
	}
}

func TestParseDicRef(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{in: "初音ミク", want: "初音ミク", ok: true},
		{in: "  Hatsune Miku  ", want: "Hatsune Miku", ok: true},
		{in: "Re:ゼロから始める異世界生活", want: "Re:ゼロから始める異世界生活", ok: true},
		{in: "https://dic.pixiv.net/a/%E5%88%9D%E9%9F%B3%E3%83%9F%E3%82%AF", want: "初音ミク", ok: true},
		{in: "https://dic.pixiv.net/en/a/Hatsune%20Miku", want: "Hatsune Miku", ok: true},
		{in: "https://dic.pixiv.net/a/KAITO/", want: "KAITO", ok: true},
		{in: "", ok: false},
		{in: "   ", ok: false},
		{in: "https://www.pixiv.net/artworks/123", ok: false},
		{in: "https://dic.pixiv.net/search?query=x", ok: false},
		{in: "https://dic.pixiv.net/a/", ok: false},
	}
	for _, tc := range cases {
		got, err := ParseDicRef(tc.in)
		if tc.ok && err != nil {
			t.Errorf("ParseDicRef(%q) error: %v", tc.in, err)
			continue
		}
		if !tc.ok {
			if err == nil {
				t.Errorf("ParseDicRef(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if got != tc.want {
			t.Errorf("ParseDicRef(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in      string
		uriType string
		id      string
		wantErr bool
	}{
		{in: "12345678", uriType: "artwork", id: "12345678"},
		{in: "https://www.pixiv.net/artworks/999", uriType: "artwork", id: "999"},
		{in: "https://www.pixiv.net/en/artworks/999", uriType: "artwork", id: "999"},
		{in: "初音ミク", uriType: "dic_article", id: "初音ミク"},
		{in: "https://dic.pixiv.net/a/KAITO", uriType: "dic_article", id: "KAITO"},
		{in: "", wantErr: true},
		{in: "https://example.com/a/x", wantErr: true},
		{in: "https://www.pixiv.net/novel/show.php?id=1", wantErr: true},
	}
	for _, tc := range cases {
		uriType, id, err := Domain{}.Classify(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Classify(%q) = (%q, %q), want an error", tc.in, uriType, id)
			}
			continue
		}
		if err != nil {
			t.Errorf("Classify(%q) error: %v", tc.in, err)
			continue
		}
		if uriType != tc.uriType || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q), want (%q, %q)", tc.in, uriType, id, tc.uriType, tc.id)
		}
	}
}

func TestLocateDicArticle(t *testing.T) {
	got, err := Domain{}.Locate("dic_article", "初音ミク")
	want := "https://dic.pixiv.net/a/%E5%88%9D%E9%9F%B3%E3%83%9F%E3%82%AF"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
	if _, err := (Domain{}).Locate("nope", "1"); err == nil {
		t.Error("Locate(nope) should fail")
	}
}

func TestDicLangs(t *testing.T) {
	if len(DicLangs) == 0 {
		t.Fatal("DicLangs is empty")
	}
	for _, l := range DicLangs {
		if !ValidDicLang(l) {
			t.Errorf("ValidDicLang(%q) = false", l)
		}
	}
	if ValidDicLang("fr") {
		t.Error("ValidDicLang(fr) should be false")
	}
}
