package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bukalapak/ottoman/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ProxySuite struct {
	suite.Suite
	backend *httptest.Server
}

func (suite *ProxySuite) SetupSuite() {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"foo":"bar"}`)
	}

	suite.backend = httptest.NewServer(http.HandlerFunc(fn))
}

func (suite *ProxySuite) TearDownSuite() {
	suite.backend.Close()
}

func (suite *ProxySuite) Targeter() proxy.Targeter {
	u, _ := url.Parse(suite.backend.URL)
	t := proxy.NewTarget(u)

	return t
}

type Director struct{}

func (s Director) Director(t proxy.Targeter) func(r *http.Request) {
	return func(r *http.Request) {
		u := t.Target()
		r.URL.Scheme = u.Scheme
		r.URL.Host = u.Host
	}
}

func (suite *ProxySuite) TestProxy() {
	req, _ := http.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	x := proxy.NewProxy(suite.Targeter())
	x.Forward(rec, req, Director{})

	assert.Equal(suite.T(), http.StatusOK, rec.Code)
	assert.Equal(suite.T(), "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(suite.T(), `{"foo":"bar"}`, strings.TrimSpace(rec.Body.String()))
}

func TestProxySuite(t *testing.T) {
	suite.Run(t, new(ProxySuite))
}
