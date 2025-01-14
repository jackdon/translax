package translator

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

var DefaultAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.146 Safari/537.36"

type EngineName string

const (
	EngineBaidu  = EngineName("baidu")
	EngineSougou = EngineName("sougou")
	EngineYoudao = EngineName("youdao")
	EngineBing   = EngineName("bing")
	EngineGoogle = EngineName("google")
)

var LangMap = map[string]string{
	"af":  "af",
	"sq":  "sq",
	"am":  "am",
	"ar":  "ar",
	"hy":  "hy",
	"as":  "as",
	"az":  "az",
	"bn":  "bn",
	"bs":  "bs",
	"bg":  "bg",
	"yue": "yue",
	"ca":  "ca",
	"zh":  "zh",
	"hr":  "hr",
	"cs":  "cs",
	"da":  "da",
	"prs": "prs",
	"nl":  "nl",
	"en":  "en",
	"et":  "et",
	"fj":  "fj",
	"fil": "fil",
	"fi":  "fi",
	"fr":  "fr",
	"de":  "de",
	"el":  "el",
	"gu":  "gu",
	"ht":  "ht",
	"he":  "he",
	"hi":  "hi",
	"mww": "mww",
	"hu":  "hu",
	"is":  "is",
	"id":  "id",
	"iu":  "iu",
	"ga":  "ga",
	"it":  "it",
	"ja":  "ja",
	"kn":  "kn",
	"kk":  "kk",
	"km":  "km",
	"tlh": "tlh",
	"ko":  "ko",
	"ku":  "ku",
	"kmr": "kmr",
	"lo":  "lo",
	"lv":  "lv",
	"lt":  "lt",
	"mg":  "mg",
	"ms":  "ms",
	"ml":  "ml",
	"mt":  "mt",
	"mi":  "mi",
	"mr":  "mr",
	"my":  "my",
	"ne":  "ne",
	"nb":  "nb",
	"or":  "or",
	"ps":  "ps",
	"fa":  "fa",
	"pl":  "pl",
	"pt":  "pt",
	"pa":  "pa",
	"otq": "otq",
	"ro":  "ro",
	"ru":  "ru",
	"sm":  "sm",
	"sr":  "sr",
	"sk":  "sk",
	"sl":  "sl",
	"es":  "es",
	"sw":  "sw",
	"sv":  "sv",
	"ty":  "ty",
	"ta":  "ta",
	"te":  "te",
	"th":  "th",
	"ti":  "ti",
	"to":  "to",
	"tr":  "tr",
	"uk":  "uk",
	"ur":  "ur",
	"vi":  "vi",
	"cy":  "cy",
	"yua": "yua",
}

type Result interface {
	fmt.Stringer
}

type Session struct {
	ExprAt  int64          `yaml:"expr_at"`
	Cookies []*http.Cookie `yaml:"cookies"`
}

type SessionCache interface {
	Persist(engine EngineName, session *Session) error
	GetSession(engine EngineName) (*Session, error)
	GetTranslatorByEngineName(engine EngineName) Translator
}

type defaultSessionCache struct {
	SessionCache
	memSession map[EngineName]*Session
}

func (c *defaultSessionCache) Load() error {
	dir, err := getDir()
	if err != nil {
		return err
	}
	for e := range ENGINES {
		d, err := ioutil.ReadFile(filepath.Join(dir, string(e)+".yaml"))
		if err != nil {
			return err
		}
		s := new(Session)
		if err := yaml.Unmarshal(d, s); err == nil {
			c.memSession[e] = s
		}
	}
	return nil
}

func getDir() (dir string, err error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir = filepath.Join(homeDir, ".config", "translaX")
	return
}
func (c *defaultSessionCache) Persist(engine EngineName, session *Session) error {
	if session == nil {
		return errors.New("session can not be nil.")
	}
	dir, err := getDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("缓存目录创建失败: %v", err)
	}
	if d, err := yaml.Marshal(session); err != nil {
		return err
	} else {
		return ioutil.WriteFile(filepath.Join(dir, string(engine)+".yaml"), d, os.ModePerm)
	}
}

func (c *defaultSessionCache) GetSession(engine EngineName) (*Session, error) {
	if s, ok := c.memSession[engine]; ok {
		return s, nil
	} else {
		t := c.GetTranslatorByEngineName(engine)
		if t == nil {
			return nil, errors.New("no translator found")
		}
		s, err := t.Session()
		if err == nil {
			c.memSession[engine] = s
			c.Persist(engine, s)
		}
		return s, err
	}
}

func (c *defaultSessionCache) GetTranslatorByEngineName(engine EngineName) Translator {
	return ENGINES[engine]
}

type Translator interface {
	Engine() EngineName
	Session() (*Session, error)
	Translate(srcLang, targetLang, text string) (Result, error)
	postForm(url string, data url.Values) (*http.Response, error)
	post(url string, data []byte) (*http.Response, error)
}

type basicTranslator struct {
	Translator
	engine EngineName
	agent  string
	cache  SessionCache
}

func (b *basicTranslator) Engine() EngineName {
	return b.engine
}

func (b *basicTranslator) addHeaders(req *http.Request) {
	req.Header.Set("User-Agent", b.agent)
	// Headers for Youdao
	if b.engine == EngineYoudao {
		req.Header.Set("Host", "fanyi.youdao.com")
		req.Header.Set("Origin", "https://fanyi.youdao.com")
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Sec-GPC", "1")
		req.Header.Set("Referer", "https://fanyi.youdao.com/")
	}
}

func (b *basicTranslator) postForm(url string, data url.Values) (resp *http.Response, err error) {
	var req *http.Request
	if req, err = http.NewRequest("POST", url, strings.NewReader(data.Encode())); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8;")
	s, err := b.cache.GetSession(b.Engine())
	if err != nil {
		return
	}
	for _, c := range s.Cookies {
		req.Header.Add("Cookie", c.Raw)
	}
	b.addHeaders(req)
	return http.DefaultClient.Do(req)
}

func (b *basicTranslator) post(url string, data []byte) (resp *http.Response, err error) {
	var req *http.Request
	if req, err = http.NewRequest("POST", url, bytes.NewBuffer(data)); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	s, err := b.cache.GetSession(b.Engine())
	if err != nil {
		return
	}
	for _, c := range s.Cookies {
		req.Header.Add("Cookie", c.Raw)
	}
	req.Header.Set("User-Agent", b.agent)
	return http.DefaultClient.Do(req)
}

func (b *basicTranslator) get(url string) (resp *http.Response, err error) {
	var req *http.Request
	if req, err = http.NewRequest("GET", url, nil); err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", b.agent)
	return http.DefaultClient.Do(req)
}

func (b *basicTranslator) keepLang(srcLang, targetLang string) (sl, tl string, err error) {
	if sl, okSl := LangMap[strings.ToLower(srcLang)]; okSl {
		if tl, okTl := LangMap[strings.ToLower(targetLang)]; okTl {
			if b.Engine() == EngineBing {
				if sl == "zh" {
					sl = "zh-Hans"
				}
				if tl == "zh" {
					tl = "zh-Hans"
				}
			}
			if b.Engine() == EngineSougou || b.Engine() == EngineYoudao {
				if sl == "zh" {
					sl = "zh-CHS"
				}
				if tl == "zh" {
					tl = "zh-CHS"
				}
			}
			return sl, tl, nil
		}
	}
	return "", "", errors.New("not supported language code.")
}

var ENGINES = map[EngineName]Translator{}

func RegisterTranslator(translator Translator) {
	ENGINES[translator.Engine()] = translator
}

var defaultCache = &defaultSessionCache{
	memSession: make(map[EngineName]*Session),
}

func init() {
	RegisterTranslator(NewSougou(defaultCache))
	RegisterTranslator(NewBing(defaultCache))
	RegisterTranslator(NewGoogle(defaultCache))
	RegisterTranslator(NewYoudao(defaultCache))
	// after register all translator
	defaultCache.Load()
}

func Trans(engine EngineName, from, to, text string) (string, error) {
	switch engine {
	case EngineGoogle:
		r, err := ENGINES[EngineGoogle].Translate(from, to, text)
		return fmt.Sprintf("%v", r), err
	case EngineBing:
		r, err := ENGINES[EngineBing].Translate(from, to, text)
		return fmt.Sprintf("%v", r), err
	case EngineSougou:
		r, err := ENGINES[EngineSougou].Translate(from, to, text)
		return fmt.Sprintf("%v", r), err
	case EngineYoudao:
		r, err := ENGINES[EngineYoudao].Translate(from, to, text)
		return fmt.Sprintf("%v", r), err
	default:
		return "", errors.New("engine not selected.")
	}
}
