package status

import (
	"bytes"
	"net/http"
	"regexp"
	"testing"

	"github.com/cli/cli/v2/internal/config"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/httpmock"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
)

func Test_NewCmdStatus(t *testing.T) {
	tests := []struct {
		name  string
		cli   string
		wants StatusOptions
	}{
		{
			name:  "no arguments",
			cli:   "",
			wants: StatusOptions{},
		},
		{
			name: "hostname set",
			cli:  "--hostname ellie.williams",
			wants: StatusOptions{
				Hostname: "ellie.williams",
			},
		},
		{
			name: "show token",
			cli:  "--show-token",
			wants: StatusOptions{
				ShowToken: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &cmdutil.Factory{}

			argv, err := shlex.Split(tt.cli)
			assert.NoError(t, err)

			var gotOpts *StatusOptions
			cmd := NewCmdStatus(f, func(opts *StatusOptions) error {
				gotOpts = opts
				return nil
			})

			// TODO cobra hack-around
			cmd.Flags().BoolP("help", "x", false, "")

			cmd.SetArgs(argv)
			cmd.SetIn(&bytes.Buffer{})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			_, err = cmd.ExecuteC()
			assert.NoError(t, err)

			assert.Equal(t, tt.wants.Hostname, gotOpts.Hostname)
		})
	}
}

func Test_statusRun(t *testing.T) {
	tests := []struct {
		name       string
		opts       *StatusOptions
		httpStubs  func(*httpmock.Registry)
		cfg        func(config.Config)
		wantErr    string
		wantErrOut *regexp.Regexp
	}{
		{
			name: "hostname set",
			opts: &StatusOptions{
				Hostname: "joel.miller",
			},
			cfg: func(c config.Config) {
				_ = c.Set("joel.miller", "oauth_token", "abc123")
				_ = c.Set("github.com", "oauth_token", "abc123")
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "api/v3/"), httpmock.ScopesResponder("repo,read:org"))
				reg.Register(
					httpmock.GraphQL(`query UserCurrent\b`),
					httpmock.StringResponse(`{"data":{"viewer":{"login":"tess"}}}`))
			},
			wantErrOut: regexp.MustCompile(`Logged in to joel.miller as.*tess`),
		},
		{
			name: "missing scope",
			opts: &StatusOptions{},
			cfg: func(c config.Config) {
				_ = c.Set("joel.miller", "oauth_token", "abc123")
				_ = c.Set("github.com", "oauth_token", "abc123")
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "api/v3/"), httpmock.ScopesResponder("repo"))
				reg.Register(httpmock.REST("GET", ""), httpmock.ScopesResponder("repo,read:org"))
				reg.Register(
					httpmock.GraphQL(`query UserCurrent\b`),
					httpmock.StringResponse(`{"data":{"viewer":{"login":"tess"}}}`))
			},
			wantErrOut: regexp.MustCompile(`joel.miller: missing required.*Logged in to github.com as.*tess`),
			wantErr:    "SilentError",
		},
		{
			name: "bad token",
			opts: &StatusOptions{},
			cfg: func(c config.Config) {
				_ = c.Set("joel.miller", "oauth_token", "abc123")
				_ = c.Set("github.com", "oauth_token", "abc123")
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "api/v3/"), httpmock.StatusStringResponse(400, "no bueno"))
				reg.Register(httpmock.REST("GET", ""), httpmock.ScopesResponder("repo,read:org"))
				reg.Register(
					httpmock.GraphQL(`query UserCurrent\b`),
					httpmock.StringResponse(`{"data":{"viewer":{"login":"tess"}}}`))
			},
			wantErrOut: regexp.MustCompile(`joel.miller: authentication failed.*Logged in to github.com as.*tess`),
			wantErr:    "SilentError",
		},
		{
			name: "all good",
			opts: &StatusOptions{},
			cfg: func(c config.Config) {
				_ = c.Set("joel.miller", "oauth_token", "abc123")
				_ = c.Set("github.com", "oauth_token", "abc123")
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "api/v3/"), httpmock.ScopesResponder("repo,read:org"))
				reg.Register(httpmock.REST("GET", ""), httpmock.ScopesResponder("repo,read:org"))
				reg.Register(
					httpmock.GraphQL(`query UserCurrent\b`),
					httpmock.StringResponse(`{"data":{"viewer":{"login":"tess"}}}`))
				reg.Register(
					httpmock.GraphQL(`query UserCurrent\b`),
					httpmock.StringResponse(`{"data":{"viewer":{"login":"tess"}}}`))
			},
			wantErrOut: regexp.MustCompile(`(?s)Logged in to github.com as.*tess.*Logged in to joel.miller as.*tess`),
		},
		{
			name: "hide token",
			opts: &StatusOptions{},
			cfg: func(c config.Config) {
				_ = c.Set("joel.miller", "oauth_token", "abc123")
				_ = c.Set("github.com", "oauth_token", "xyz456")
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "api/v3/"), httpmock.ScopesResponder("repo,read:org"))
				reg.Register(httpmock.REST("GET", ""), httpmock.ScopesResponder("repo,read:org"))
				reg.Register(
					httpmock.GraphQL(`query UserCurrent\b`),
					httpmock.StringResponse(`{"data":{"viewer":{"login":"tess"}}}`))
				reg.Register(
					httpmock.GraphQL(`query UserCurrent\b`),
					httpmock.StringResponse(`{"data":{"viewer":{"login":"tess"}}}`))
			},
			wantErrOut: regexp.MustCompile(`(?s)Token: \*{19}.*Token: \*{19}`),
		},
		{
			name: "show token",
			opts: &StatusOptions{
				ShowToken: true,
			},
			cfg: func(c config.Config) {
				_ = c.Set("joel.miller", "oauth_token", "abc123")
				_ = c.Set("github.com", "oauth_token", "xyz456")
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "api/v3/"), httpmock.ScopesResponder("repo,read:org"))
				reg.Register(httpmock.REST("GET", ""), httpmock.ScopesResponder("repo,read:org"))
				reg.Register(
					httpmock.GraphQL(`query UserCurrent\b`),
					httpmock.StringResponse(`{"data":{"viewer":{"login":"tess"}}}`))
				reg.Register(
					httpmock.GraphQL(`query UserCurrent\b`),
					httpmock.StringResponse(`{"data":{"viewer":{"login":"tess"}}}`))
			},
			wantErrOut: regexp.MustCompile(`(?s)Token: xyz456.*Token: abc123`),
		}, {
			name: "missing hostname",
			opts: &StatusOptions{
				Hostname: "github.example.com",
			},
			cfg: func(c config.Config) {
				_ = c.Set("github.com", "oauth_token", "abc123")
			},
			httpStubs:  func(reg *httpmock.Registry) {},
			wantErrOut: regexp.MustCompile(`(?s)Hostname "github.example.com" not found among authenticated GitHub hosts`),
			wantErr:    "SilentError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts == nil {
				tt.opts = &StatusOptions{}
			}

			ios, _, _, stderr := iostreams.Test()

			ios.SetStdinTTY(true)
			ios.SetStderrTTY(true)
			ios.SetStdoutTTY(true)

			tt.opts.IO = ios

			cfg := config.NewBlankConfig()

			if tt.cfg != nil {
				tt.cfg(cfg)
			}
			tt.opts.Config = func() (config.Config, error) {
				return cfg, nil
			}

			reg := &httpmock.Registry{}
			tt.opts.HttpClient = func() (*http.Client, error) {
				return &http.Client{Transport: reg}, nil
			}
			if tt.httpStubs != nil {
				tt.httpStubs(reg)
			}
			mainBuf := bytes.Buffer{}
			hostsBuf := bytes.Buffer{}
			defer config.StubWriteConfig(&mainBuf, &hostsBuf)()

			err := statusRun(tt.opts)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			} else {
				assert.NoError(t, err)
			}

			if tt.wantErrOut == nil {
				assert.Equal(t, "", stderr.String())
			} else {
				assert.True(t, tt.wantErrOut.MatchString(stderr.String()))
			}

			assert.Equal(t, "", mainBuf.String())
			assert.Equal(t, "", hostsBuf.String())

			reg.Verify(t)
		})
	}
}
