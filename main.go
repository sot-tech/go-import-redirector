/*
 * BSD-3-Clause
 * Copyright 2021 sot (aka PR_713, C_rho_272)
 * Redistribution and use in source and binary forms, with or without modification,
 * are permitted provided that the following conditions are met:
 * 1. Redistributions of source code must retain the above copyright notice,
 * this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright notice,
 * this list of conditions and the following disclaimer in the documentation and/or
 * other materials provided with the distribution.
 * 3. Neither the name of the copyright holder nor the names of its contributors
 * may be used to endorse or promote products derived from this software without
 * specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
 * IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
 * BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA,
 * OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
 * WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY
 * OF SUCH DAMAGE.
 */

package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"text/template"
	"time"
)

const (
	pImport       = "import"
	pVcs          = "vcs"
	pSources      = "sources"
	pRedirect     = "redirect"
	pRedirectDir  = "redirectDir"
	pRedirectFile = "redirectFile"
	qGoGet        = "go-get"
	timeout       = time.Second
)

var tmpl = template.Must(template.New("main").Parse(`<!DOCTYPE html>
<html>
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
<meta name="go-import" content="{{.` + pImport + `}} {{.` + pVcs + `}} {{.` + pSources + `}}">
<meta name="go-source" content="{{.` + pImport + `}} {{.` + pSources + `}}{{.` + pRedirectDir + `}}{{.` + pRedirectFile + `}}">
<meta http-equiv="refresh" content="0; url={{.` + pRedirect + `}}">
</head>
<body>
Redirecting to docs at <a href="{{.` + pRedirect + `}}">{{.` + pRedirect + `}}</a>...
</body>
</html>
`))

type Handler struct {
	prefix                                *url.URL
	vcs                                   string
	redirect                              *url.URL
	redirectDirSuffix, redirectFileSuffix string
	forwardedHeader                       string
}

func New(prefix, vcs, redirect, redirectDir, redirectFile, forwardedHeader string) (h Handler, err error) {
	var p Handler
	if p.prefix, err = url.Parse(prefix); err != nil {
		return
	}
	if p.redirect, err = url.Parse(redirect); err != nil {
		return
	}
	if strings.HasPrefix(p.redirect.Scheme, "http") {
		p.redirectDirSuffix = redirectDir
		p.redirectFileSuffix = redirectFile
	}
	if len(forwardedHeader) > 0 {
		p.forwardedHeader = http.CanonicalHeaderKey("forwardedHeader")
	}
	p.vcs = vcs
	h = p
	return
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, p := r.URL, strings.TrimSuffix(r.URL.EscapedPath(), "/")
	if v, exist := u.Query()[qGoGet]; exist && len(v) == 1 && v[0] == "1" && len(p) > 0 {
		base, project := path.Split(p)
		if len(base) == 0 || base == "/" {
			if len(u.Host) == 0 {
				fwd := r.Header.Get(h.forwardedHeader)
				if len(fwd) > 0 {
					r.Host = fwd
				}
				if sep := strings.IndexRune(r.Host, ':'); sep > 0 {
					base = r.Host[:sep]
				} else {
					base = r.Host
				}
			} else {
				base = u.Host
			}
		}
		base = path.Join(base, project)
		replacements := map[string]string{
			pImport: strings.TrimPrefix(base, "/"),
			pVcs:    h.vcs,
		}
		var redirect *url.URL
		var err error
		if redirect, err = h.prefix.Parse(path.Join(h.prefix.Path, base)); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		replacements[pSources] = redirect.String()
		if redirect, err = h.redirect.Parse(path.Join(h.redirect.Path, base)); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		baseRedirect := redirect.String()
		replacements[pRedirect] = baseRedirect
		if len(h.redirectDirSuffix) > 0 {
			replacements[pRedirectDir] = " " + path.Join(baseRedirect, h.redirectDirSuffix)
		}
		if len(h.redirectFileSuffix) > 0 {
			replacements[pRedirectFile] = " " + path.Join(baseRedirect, h.redirectFileSuffix)
		}
		_ = tmpl.Execute(w, replacements)
	} else {
		http.NotFound(w, r)
	}
}

func main() {
	listen := flag.String("l", ":http", "listen address")
	prefix := flag.String("p", "https://github.com", "vcs prefix")
	vcs := flag.String("t", "git", "set version control system")
	redirect := flag.String("r", "https://pkg.go.dev", "redirect prefix")
	redirectDir := flag.String("rd", "/tree/master{/dir}", "redirect suffix dir")
	redirectFile := flag.String("rf", "/blob/master{/dir}/{file}#L{line}", "redirect suffix file")
	forwardedHeader := flag.String("x", "X-Forwarded-Host", "custom header of real address")
	log.SetPrefix("gir: ")
	flag.Parse()
	if handler, err := New(*prefix, *vcs, *redirect, *redirectDir, *redirectFile, *forwardedHeader); err != nil {
		log.Fatal(err)
	} else {
		s := http.Server{
			Addr:         *listen,
			Handler:      handler,
			ReadTimeout:  timeout,
			WriteTimeout: timeout,
		}
		go func() {
			if err = s.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				log.Fatal(err)
			}
		}()
		defer s.Close()
		ch := make(chan os.Signal, 2)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
	}
}
