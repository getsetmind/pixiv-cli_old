package pixiv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// DicHost is the host of the Pixiv encyclopedia.
const DicHost = "dic.pixiv.net"

// DicBaseURL is the root every encyclopedia request is built from.
const DicBaseURL = "https://" + DicHost

// dicReferer is sent with every encyclopedia request.
const dicReferer = DicBaseURL + "/"

// dicTime is the zone the encyclopedia renders its timestamps in.
var dicTime = time.FixedZone("JST", 9*60*60)

// DicArticle is one hit from a Pixiv encyclopedia search.
type DicArticle struct {
	Title      string `json:"title"                kit:"id" table:"title"`
	Summary    string `json:"summary,omitempty"              table:"summary,truncate"`
	Updated    string `json:"updated,omitempty"              table:"updated"`
	Views      int    `json:"views"                          table:"views"`
	Works      int    `json:"works"                          table:"works"`
	Checklists int    `json:"checklists"                     table:"checklists"`
	Related    string `json:"related,omitempty"              table:"related,truncate"`
	URL        string `json:"url"                            table:"url,url"`
	Thumbnail  string `json:"thumbnail,omitempty"            table:"-"`
}

// DicEntry is one Pixiv encyclopedia article.
type DicEntry struct {
	ID          int      `json:"id"                   kit:"id" table:"id"`
	Title       string   `json:"title"                        table:"title"`
	Yomigana    string   `json:"yomigana,omitempty"            table:"yomigana"`
	Translation string   `json:"translation,omitempty"         table:"translation"`
	Categories  string   `json:"categories,omitempty"          table:"categories"`
	Abstract    string   `json:"abstract,omitempty"            table:"abstract,truncate"`
	Related     string   `json:"related,omitempty"             table:"related,truncate"`
	Views       int      `json:"views"                         table:"views"`
	Works       int      `json:"works"                         table:"works"`
	Comments    int      `json:"comments"                      table:"comments"`
	Checklists  int      `json:"checklists"                    table:"checklists"`
	Updated     string   `json:"updated,omitempty"             table:"updated"`
	URL         string   `json:"url"                           table:"url,url"`
	Tags        []string `json:"tags,omitempty"                table:"-"`
	Thumbnail   string   `json:"thumbnail,omitempty"           table:"-"`
	Body        string   `json:"body,omitempty"                table:"-"`
}

// DicLangs are the article languages the encyclopedia serves.
var DicLangs = []string{"ja", "en"}

// ValidDicLang reports whether lang is an article language the encyclopedia
// serves.
func ValidDicLang(lang string) bool {
	for _, l := range DicLangs {
		if l == lang {
			return true
		}
	}
	return false
}

// ParseDicRef accepts an encyclopedia reference -- a bare article title or a
// dic.pixiv.net URL -- and returns the article title.
func ParseDicRef(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", fmt.Errorf("empty article title")
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return s, nil // a bare title, not a URL
	}
	if !strings.EqualFold(u.Hostname(), DicHost) {
		return "", fmt.Errorf("not a %s URL: %q", DicHost, input)
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, seg := range segs {
		if seg != "a" {
			continue
		}
		title := strings.TrimSpace(strings.Join(segs[i+1:], "/"))
		if title == "" {
			break
		}
		return title, nil
	}
	return "", fmt.Errorf("no article title in %q", input)
}

// SearchArticles searches the encyclopedia, returning one page of hits.
func (c *Client) SearchArticles(ctx context.Context, query string, page int) ([]DicArticle, error) {
	if page <= 0 {
		page = 1
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("p", strconv.Itoa(page))
	rawURL := c.dicBaseURL + "/search?" + q.Encode()

	body, err := c.get(ctx, rawURL, "text/html", dicReferer)
	if err != nil {
		// The site answers a search that matched nothing with 404 and a valid
		// "0 results" page, so a 404 here means empty, not broken.
		if isHTTPStatus(err, http.StatusNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return parseDicSearch(body)
}

// Article fetches one encyclopedia article and its counters. lang selects the
// article language and must be one of DicLangs.
func (c *Client) Article(ctx context.Context, title, lang string) (*DicEntry, error) {
	if lang == "" {
		lang = DicLangs[0]
	}
	esc := url.PathEscape(title)
	query := "?lang=" + url.QueryEscape(lang)

	raw, err := c.get(ctx, c.dicBaseURL+"/_api/get_article/"+esc+query, "application/json", dicReferer)
	if err != nil {
		return nil, err
	}
	var w wireDicArticle
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, fmt.Errorf("decode article: %w", err)
	}

	// The counters are a second call; treat them as optional so a hiccup there
	// still yields the article itself.
	var info wireDicInfo
	if rawInfo, err := c.get(ctx, c.dicBaseURL+"/_api/get_article_info/"+esc+query, "application/json", dicReferer); err == nil {
		_ = json.Unmarshal(rawInfo, &info)
	}

	e := &DicEntry{
		ID:          w.ID,
		Title:       w.TagName,
		Yomigana:    w.Yomigana,
		Translation: w.TranslatedTagName,
		Categories:  strings.Join(w.Categories, ", "),
		Abstract:    w.Abstract,
		Related:     joinDicTagNames(w.RecommendedArticles),
		Tags:        w.TagNameListInArticleJa,
		Body:        dicNodesText(w.Nodes),
		Views:       info.ArticleViewCount,
		Works:       info.PixivWorkCount,
		Comments:    info.CommentCount,
		Checklists:  info.ChecklistCount,
		URL:         dicArticleURL(lang, w.TagName),
	}
	if w.UpdatedAtTimestamp > 0 {
		e.Updated = time.Unix(w.UpdatedAtTimestamp, 0).In(dicTime).Format("2006-01-02 15:04:05")
	}
	if w.MainIllust != nil {
		e.Thumbnail = w.MainIllust.ImageURL
	}
	return e, nil
}

// dicArticleURL is the live page for an article title in one language.
func dicArticleURL(lang, title string) string {
	prefix := "/a/"
	if lang != "" && lang != DicLangs[0] {
		prefix = "/" + lang + "/a/"
	}
	return DicBaseURL + prefix + url.PathEscape(title)
}

// joinDicTagNames renders a related-article list as a comma-separated string.
func joinDicTagNames(arts []wireDicRelated) string {
	names := make([]string, 0, len(arts))
	for _, a := range arts {
		if a.TagName != "" {
			names = append(names, a.TagName)
		}
	}
	return strings.Join(names, ", ")
}

// --- search page scraping ---------------------------------------------------

// parseDicSearch extracts the article cards from a /search page.
func parseDicSearch(body []byte) ([]DicArticle, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse search page: %w", err)
	}
	main := findDicElement(doc, func(n *html.Node) bool {
		return isDicElem(n, "div") && dicAttr(n, "id") == "main"
	})
	if main == nil {
		return nil, fmt.Errorf("parse search page: no result container")
	}

	var out []DicArticle
	for _, card := range findDicElements(main, func(n *html.Node) bool { return isDicElem(n, "article") }) {
		if a, ok := parseDicCard(card); ok {
			out = append(out, a)
		}
	}
	return out, nil
}

// parseDicCard reads one <article> card from a search result page.
func parseDicCard(card *html.Node) (DicArticle, bool) {
	info := findDicElement(card, func(n *html.Node) bool {
		return isDicElem(n, "div") && dicHasClass(n, "info")
	})
	if info == nil {
		return DicArticle{}, false
	}
	link := findDicElement(info, func(n *html.Node) bool {
		return isDicElem(n, "a") && dicAttr(n, "href") != ""
	})
	if link == nil {
		return DicArticle{}, false
	}

	a := DicArticle{
		Title: dicText(link),
		URL:   dicAbsolute(dicAttr(link, "href")),
	}
	if thumb := findDicElement(card, func(n *html.Node) bool { return isDicElem(n, "img") }); thumb != nil {
		a.Thumbnail = dicAttr(thumb, "src")
	}
	if p := findDicElement(info, func(n *html.Node) bool {
		return isDicElem(n, "p") && dicHasClass(n, "summary")
	}); p != nil {
		a.Summary = dicTextBeforeLink(p)
	}
	if ul := findDicElement(info, func(n *html.Node) bool {
		return isDicElem(n, "ul") && dicHasClass(n, "data")
	}); ul != nil {
		parseDicStats(ul, &a)
	}
	if rel := findDicElement(info, func(n *html.Node) bool {
		return isDicElem(n, "div") && dicHasClass(n, "relation")
	}); rel != nil {
		var names []string
		for _, li := range findDicElements(rel, func(n *html.Node) bool { return isDicElem(n, "li") }) {
			if name := dicText(li); name != "" {
				names = append(names, name)
			}
		}
		a.Related = strings.Join(names, ", ")
	}
	if a.Title == "" {
		return DicArticle{}, false
	}
	return a, true
}

// parseDicStats reads the labelled counter lines under an article card.
func parseDicStats(ul *html.Node, a *DicArticle) {
	for _, li := range findDicElements(ul, func(n *html.Node) bool { return isDicElem(n, "li") }) {
		label, value, ok := strings.Cut(dicText(li), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(label) {
		case "更新":
			a.Updated = strings.TrimSpace(value)
		case "閲覧数":
			a.Views = dicAtoi(value)
		case "作品数":
			a.Works = dicAtoi(value)
		case "チェックリスト数":
			a.Checklists = dicAtoi(value)
		}
	}
}

// dicAbsolute turns a site-relative href into an absolute URL.
func dicAbsolute(href string) string {
	if href == "" || strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	return DicBaseURL + "/" + strings.TrimPrefix(href, "/")
}

// dicAtoi parses a display integer, tolerating thousands separators.
func dicAtoi(s string) int {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	n, _ := strconv.Atoi(digits)
	return n
}

// --- html helpers -----------------------------------------------------------

func isDicElem(n *html.Node, name string) bool {
	return n.Type == html.ElementNode && n.Data == name
}

func dicAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func dicHasClass(n *html.Node, class string) bool {
	for _, f := range strings.Fields(dicAttr(n, "class")) {
		if f == class {
			return true
		}
	}
	return false
}

// findDicElement returns the first element satisfying pred, in document order.
func findDicElement(root *html.Node, pred func(*html.Node) bool) *html.Node {
	if root.Type == html.ElementNode && pred(root) {
		return root
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if found := findDicElement(c, pred); found != nil {
			return found
		}
	}
	return nil
}

// findDicElements returns every element satisfying pred, but does not descend
// into a match, so nested containers are not reported twice.
func findDicElements(root *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && pred(n) {
			out = append(out, n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

// dicText is the whitespace-collapsed text of n and its descendants.
func dicText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// dicTextBeforeLink is the text of n up to its first <a>, so a summary that
// ends in the site's "read more" link keeps only the prose.
func dicTextBeforeLink(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node) bool
	walk = func(x *html.Node) bool {
		if isDicElem(x, "a") {
			return false
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			if !walk(c) {
				return false
			}
		}
		return true
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// --- article body -----------------------------------------------------------

// dicNode is one node of the article body tree. The API ships the body as a
// JSON string holding an array of these.
type dicNode struct {
	Tag      string     `json:"tag"`
	Text     string     `json:"text"`
	Children []*dicNode `json:"children"`
}

// dicNodesText renders the article body tree as plain text.
func dicNodesText(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var nodes []*dicNode
	if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
		return ""
	}
	var b strings.Builder
	writeDicNodes(&b, nodes)
	return dicTidy(b.String())
}

func writeDicNodes(b *strings.Builder, nodes []*dicNode) {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		switch n.Tag {
		case "text", "article_link", "external_link":
			b.WriteString(n.Text)
		case "br":
			b.WriteString("\n")
		case "header":
			b.WriteString("\n\n## ")
			writeDicNodes(b, n.Children)
			b.WriteString("\n")
		case "sub_header":
			b.WriteString("\n\n### ")
			writeDicNodes(b, n.Children)
			b.WriteString("\n")
		case "p":
			b.WriteString("\n\n")
			writeDicNodes(b, n.Children)
			b.WriteString("\n")
		case "list_item":
			b.WriteString("\n- ")
			writeDicNodes(b, n.Children)
		case "table_row":
			b.WriteString("\n")
			writeDicNodes(b, n.Children)
		case "table_header", "table_cell":
			writeDicNodes(b, n.Children)
			b.WriteString(" | ")
		default:
			writeDicNodes(b, n.Children)
		}
	}
}

// dicTidy trims trailing blanks and collapses runs of blank lines.
func dicTidy(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	blank := true
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if blank {
				continue
			}
			blank = true
			out = append(out, "")
			continue
		}
		blank = false
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// --- wire types -------------------------------------------------------------

type wireDicArticle struct {
	ID                     int              `json:"id"`
	TagName                string           `json:"tagName"`
	TranslatedTagName      string           `json:"translatedTagName"`
	Yomigana               string           `json:"yomigana"`
	Abstract               string           `json:"abstract"`
	Categories             []string         `json:"categories"`
	Nodes                  string           `json:"nodes"`
	TagNameListInArticleJa []string         `json:"tagNameListInArticleJa"`
	RecommendedArticles    []wireDicRelated `json:"recommendedArticles"`
	MainIllust             *wireDicIllust   `json:"mainIllust"`
	UpdatedAtTimestamp     int64            `json:"updatedAtTimestamp"`
}

type wireDicInfo struct {
	ArticleViewCount int `json:"articleViewCount"`
	CommentCount     int `json:"commentCount"`
	PixivWorkCount   int `json:"pixivWorkCount"`
	ChecklistCount   int `json:"checklistCount"`
}

type wireDicRelated struct {
	TagName string `json:"tagName"`
}

type wireDicIllust struct {
	ImageURL string `json:"imageUrl"`
}
