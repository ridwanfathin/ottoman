package cache

import (
	"errors"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/bukalapak/ottoman/encoding/json"
	"go.uber.org/zap"
)

type Engine struct {
	name      string
	engine    Reader
	Prefix    string
	Resolver  Resolver
	Timeout   time.Duration
	Transport http.RoundTripper
	Logger    *zap.Logger
}

func NewProvider(r Reader) Provider {
	return &Engine{engine: r, name: r.Name(), Logger: zap.New(nil)}
}

// Name returns cache backend identifier.
func (s *Engine) Name() string {
	return s.name
}

// Namespace returns cache Prefix
func (s *Engine) Namespace() string {
	return s.Prefix
}

// Read reads cache data on the cache backend based on key supplied.
func (s *Engine) Read(key string) ([]byte, error) {
	return s.engine.Read(s.Normalize(key))
}

// ReadMap reads cache data as map[string]interface{}.
// It's also expand any cache identifier with actual cache data.
func (s *Engine) ReadMap(key string) (map[string]interface{}, error) {
	return s.engine.ReadMap(s.Normalize(key))
}

// ReadMulti bulk reads multiple cache keys.
func (s *Engine) ReadMulti(keys []string) (map[string][]byte, error) {
	return s.engine.ReadMulti(s.NormalizeMulti(keys))
}

func (s *Engine) Fetch(key string, r *http.Request) ([]byte, error) {
	req, err := s.Resolver.Resolve(key, r)
	if err != nil {
		return nil, err
	}

	return s.FetchRequest(req)
}

func (s *Engine) FetchMap(key string, r *http.Request) (map[string]interface{}, error) {
	b, err := s.Fetch(key, r)
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})

	if err = json.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	return m, nil
}

func (s *Engine) FetchMulti(keys []string, r *http.Request) (map[string][]byte, error) {
	ks := s.NormalizeMulti(keys)
	mb := make(map[string][]byte, len(ks))
	mx := &sync.Mutex{}

	var wg sync.WaitGroup

	for _, k := range ks {
		wg.Add(1)

		go func(key string) {
			defer wg.Done()

			z, err := s.Fetch(key, r)
			if err != nil {
				s.Logger.Info("ottoman/cache",
					zap.String("method", "Fetch"),
					zap.String("error", err.Error()),
				)
			}

			mx.Lock()
			mb[key] = z
			mx.Unlock()
		}(k)
	}

	wg.Wait()

	return mb, nil
}

func (s *Engine) Normalize(key string) string {
	return Normalize(key, s.Prefix)
}

func (s *Engine) NormalizeMulti(keys []string) []string {
	ks := make([]string, len(keys))

	for i := range keys {
		ks[i] = s.Normalize(keys[i])
	}

	return ks
}

func (s *Engine) FetchRequest(r *http.Request) ([]byte, error) {
	c := s.httpClient()

	resp, err := c.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	s.Logger.Info("ottoman/cache",
		zap.String("request_url", r.URL.String()),
		zap.Int("request_status", resp.StatusCode),
	)

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("invalid http status: " + resp.Status)
	}

	return ioutil.ReadAll(resp.Body)
}

func (s *Engine) httpClient() *http.Client {
	return &http.Client{
		Transport: s.httpTransport(),
		Timeout:   s.Timeout,
	}
}

func (s *Engine) httpTransport() http.RoundTripper {
	if s.Transport != nil {
		return s.Transport
	}

	return http.DefaultTransport
}
