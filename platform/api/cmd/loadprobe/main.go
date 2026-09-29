// Command loadprobe runs a bounded, representative load against a BrambiLab origin and reports
// latency percentiles per endpoint (web-v1.md §15.3, WEB-008). It measures; it does not promise
// capacity. Only localhost by default: a real origin needs -allow-remote (and authorization).
//
//	go run ./cmd/loadprobe -base http://localhost:8000 -c 4 -d 30s -media <assetId>
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type target struct {
	name, method, path string
	header             http.Header
	body               func(i int) (string, string) // content type, body
}

func main() {
	base := flag.String("base", "http://localhost:8000", "origin to probe")
	conc := flag.Int("c", 4, "concurrent clients (max 16)")
	dur := flag.Duration("d", 30*time.Second, "duration (max 120s)")
	media := flag.String("media", "", "public asset id for the Range request (optional)")
	query := flag.String("q", "rover", "search text")
	contact := flag.Bool("contact", false, "include contact posts (only against a stack with the Resend double)")
	remote := flag.Bool("allow-remote", false, "allow a non-localhost origin (needs authorization)")
	flag.Parse()
	u, err := url.Parse(*base)
	if err != nil || u.Host == "" {
		fail("invalid -base")
	}
	if h := u.Hostname(); h != "localhost" && h != "127.0.0.1" && !*remote {
		fail("refusing a non-localhost origin without -allow-remote")
	}
	if *contact && *remote {
		// It would consume the real quotas and send real e-mails: only against the Resend double.
		fail("-contact is only allowed against a local stack with the Resend double")
	}
	*conc = min(max(*conc, 1), 16)
	*dur = min(*dur, 120*time.Second)

	targets := []target{
		{name: "ssr home", method: "GET", path: "/es"},
		{name: "ssr list", method: "GET", path: "/es/proyectos"},
		{name: "api search", method: "GET", path: "/api/v1/public/es/search?q=" + url.QueryEscape(*query)},
		{name: "sitemap", method: "GET", path: "/sitemap.xml"},
	}
	if *media != "" {
		targets = append(targets, target{name: "media range", method: "GET", path: "/media/" + *media, header: http.Header{"Range": {"bytes=0-65535"}}})
	}
	if *contact {
		targets = append(targets, target{name: "contact post", method: "POST", path: "/api/v1/contact", body: func(i int) (string, string) {
			return "application/json", fmt.Sprintf(`{"name":"Carga","email":"carga@example.org","message":"Mensaje de prueba de carga %d","locale":"es","key":"load-%d-%d-padding-xx"}`, i, time.Now().UnixNano(), i)
		}})
	}

	type sample struct {
		d       time.Duration
		ok      bool
		limited bool // 429: the quotas working, not a failure
	}
	var mu sync.Mutex
	results := map[string][]sample{}
	client := &http.Client{Timeout: 30 * time.Second}
	stop := time.Now().Add(*dur)
	var wg sync.WaitGroup
	for w := 0; w < *conc; w++ {
		wg.Go(func() {
			for i := 0; time.Now().Before(stop); i++ {
				tg := targets[(w+i)%len(targets)]
				var body io.Reader
				req, _ := http.NewRequest(tg.method, *base+tg.path, nil)
				if tg.body != nil {
					ct, b := tg.body(i*100 + w)
					body = strings.NewReader(b)
					req, _ = http.NewRequest(tg.method, *base+tg.path, body)
					req.Header.Set("Content-Type", ct)
					req.Header.Set("Origin", *base)
					// Distinct documentation-range clients so the per-client quota is not the bottleneck.
					req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", (i*7+w)%250+1))
				}
				for k, v := range tg.header {
					req.Header[k] = v
				}
				t0 := time.Now()
				resp, err := client.Do(req)
				ok := err == nil && resp.StatusCode < 500 && resp.StatusCode != 429
				limited := err == nil && resp.StatusCode == 429
				if err == nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
				mu.Lock()
				results[tg.name] = append(results[tg.name], sample{time.Since(t0), ok, limited})
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	fmt.Printf("loadprobe %s · %d clients · %s\n%-14s %7s %7s %8s %8s %8s %8s\n", *base, *conc, *dur, "endpoint", "reqs", "errors", "limited", "p50", "p95", "p99")
	for _, tg := range targets {
		s := results[tg.name]
		if len(s) == 0 {
			continue
		}
		ds := make([]time.Duration, 0, len(s))
		errs, limited := 0, 0
		for _, x := range s {
			ds = append(ds, x.d)
			switch {
			case x.limited:
				limited++
			case !x.ok:
				errs++
			}
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		p := func(q float64) time.Duration { return ds[min(len(ds)-1, int(q*float64(len(ds))))] }
		fmt.Printf("%-14s %7d %7d %8d %8s %8s %8s\n", tg.name, len(s), errs, limited, p(0.5).Round(time.Millisecond), p(0.95).Round(time.Millisecond), p(0.99).Round(time.Millisecond))
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "loadprobe:", msg)
	os.Exit(2)
}
