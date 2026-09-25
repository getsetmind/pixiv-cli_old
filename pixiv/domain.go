package pixiv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the pixiv kit driver.
type Domain struct{}

// Info describes the scheme, hostnames, and binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "pixiv",
		Hosts:  []string{Host, DicHost},
		Identity: kit.Identity{
			Binary: "pixiv",
			Short:  "A command line for Pixiv artwork rankings and encyclopedia entries.",
			Long: `A command line for Pixiv artwork rankings and encyclopedia entries.

pixiv reads public Pixiv data over plain HTTPS and prints output that pipes
into the rest of your tools. No API key or account required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/pixiv-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)
	app.CommandGroup("dic", "Look up the Pixiv encyclopedia (dic.pixiv.net)")

	kit.Handle(app, kit.OpMeta{Name: "ranking", Group: "read", List: true,
		Summary: "Fetch the Pixiv illustration ranking",
		Args: []kit.Arg{
			{Name: "mode", Help: "ranking mode: daily, weekly, monthly, rookie, original (default: daily)", Optional: true},
		},
	}, listRanking)

	kit.Handle(app, kit.OpMeta{Name: "modes", Group: "read", List: true,
		Summary: "List available ranking modes and content types",
	}, listModes)

	// Nested verbs carry no Group: kit only defines help groups on the root
	// command, and cobra rejects a group id that its parent has not declared.
	kit.Handle(app, kit.OpMeta{Name: "search", Parent: "dic",
		Summary: "Search encyclopedia articles",
		Args:    []kit.Arg{{Name: "query", Help: "words to search for"}},
	}, searchDic)

	kit.Handle(app, kit.OpMeta{Name: "article", Parent: "dic", Single: true,
		URIType: "dic_article", Resolver: true,
		Summary: "Fetch one encyclopedia article",
		Args:    []kit.Arg{{Name: "title", Help: "article title, or a dic.pixiv.net URL"}},
	}, getDicArticle)
}

// newClient builds the client from kit config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type rankingInput struct {
	Mode    string  `kit:"arg" help:"ranking mode: daily, weekly, monthly, rookie, original"`
	Content string  `kit:"flag" help:"content type: illust, manga, ugoira (default: illust)"`
	Page    int     `kit:"flag" help:"page number (default: 1)"`
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Client  *Client `kit:"inject"`
}

type modesInput struct{}

type dicSearchInput struct {
	Query  string  `kit:"arg" help:"words to search for"`
	Page   int     `kit:"flag" help:"page number (default: 1)"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type dicArticleInput struct {
	Title      string  `kit:"arg" help:"article title, or a dic.pixiv.net URL"`
	Lang       string  `kit:"flag" default:"ja" enum:"ja,en" help:"article language"`
	NoCounters bool    `kit:"flag,name=no-counters" help:"skip the view and work counters"`
	Client     *Client `kit:"inject"`
}

// --- handlers ---

func listRanking(ctx context.Context, in rankingInput, emit func(*Illust) error) error {
	mode := in.Mode
	if mode == "" {
		mode = "daily"
	}
	content := in.Content
	if content == "" {
		content = "illust"
	}
	page := in.Page
	if page <= 0 {
		page = 1
	}

	illusts, err := in.Client.Ranking(ctx, mode, content, page, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range illusts {
		if err := emit(&illusts[i]); err != nil {
			return err
		}
	}
	return nil
}

func listModes(_ context.Context, _ modesInput, emit func(*ModeInfo) error) error {
	for _, m := range Modes() {
		mc := m
		if err := emit(&mc); err != nil {
			return err
		}
	}
	return nil
}

func searchDic(ctx context.Context, in dicSearchInput, emit func(*DicArticle) error) error {
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return errs.Usage("pass the words to search for")
	}
	page := in.Page
	if page <= 0 {
		page = 1
	}

	articles, err := in.Client.SearchArticles(ctx, query, page)
	if err != nil {
		return mapErr(err)
	}
	for i := range articles {
		if err := emit(&articles[i]); err != nil {
			return err
		}
	}
	return nil
}

func getDicArticle(ctx context.Context, in dicArticleInput, emit func(*DicEntry) error) error {
	title, err := ParseDicRef(in.Title)
	if err != nil {
		return errs.Usage("%v", err)
	}
	lang := in.Lang
	if lang == "" {
		lang = DicLangs[0]
	}
	if !ValidDicLang(lang) {
		return errs.Usage("unknown language %q, want one of %s", lang, strings.Join(DicLangs, ", "))
	}

	opts := []ArticleOption{}
	if in.NoCounters {
		opts = append(opts, WithoutCounters())
	}

	entry, err := in.Client.Article(ctx, title, lang, opts...)
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return errs.NotFound("no encyclopedia article for %q in %s", title, lang)
		}
		return mapErr(err)
	}
	return emit(entry)
}

// Classify turns any accepted input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", "", errs.Usage("pass an artwork id or a dic.pixiv.net article URL")
	}

	u, parseErr := url.Parse(s)
	if parseErr == nil && u.Host != "" {
		host := strings.ToLower(u.Hostname())
		switch {
		case host == DicHost || strings.HasSuffix(host, "."+DicHost):
			title, err := ParseDicRef(s)
			if err != nil {
				return "", "", errs.Usage("%v", err)
			}
			return "dic_article", title, nil
		case host == "pixiv.net" || strings.HasSuffix(host, ".pixiv.net"):
			if id, ok := artworkID(u.Path); ok {
				return "artwork", id, nil
			}
			return "", "", errs.Usage("no artwork id in %q", input)
		default:
			return "", "", errs.Usage("pixiv does not serve %q", host)
		}
	}

	if _, err := strconv.Atoi(s); err == nil {
		return "artwork", s, nil
	}
	return "dic_article", s, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "artwork":
		return fmt.Sprintf("%s/en/artworks/%s", BaseURL, id), nil
	case "dic_article":
		return DicBaseURL + "/a/" + url.PathEscape(id), nil
	default:
		return "", errs.Usage("pixiv has no resource type %q", uriType)
	}
}

// artworkID pulls the numeric id out of a /artworks/ID path.
func artworkID(p string) (string, bool) {
	segs := strings.Split(strings.Trim(p, "/"), "/")
	for i, seg := range segs {
		if seg != "artworks" || i+1 >= len(segs) {
			continue
		}
		if _, err := strconv.Atoi(segs[i+1]); err == nil {
			return segs[i+1], true
		}
	}
	return "", false
}

// isHTTPStatus reports whether err is an HTTPError with the given status.
func isHTTPStatus(err error, code int) bool {
	var he *HTTPError
	return errors.As(err, &he) && he.StatusCode == code
}

// mapErr converts a library error into the kit error kind.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var he *HTTPError
	if errors.As(err, &he) {
		switch {
		case he.StatusCode == http.StatusNotFound:
			return errs.NotFound("%v", err)
		case he.StatusCode == http.StatusTooManyRequests:
			return errs.RateLimited("%v", err)
		case he.StatusCode >= 500:
			return errs.Network("%v", err)
		default:
			return errs.Wrap(errs.KindGeneric, err, "pixiv")
		}
	}
	var ue *url.Error
	if errors.As(err, &ue) && !errors.Is(err, context.Canceled) {
		return errs.Network("%v", err)
	}
	return err
}
