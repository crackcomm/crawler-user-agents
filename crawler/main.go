package main

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"

	"golang.org/x/net/context"

	"github.com/PuerkitoBio/goquery"
	"github.com/crackcomm/crawl"
	"github.com/golang/glog"
)

var outputFile = flag.String("output", "", "output file")

func init() {
	flag.Set("logtostderr", "true")
}

func main() {
	defer glog.Flush()
	flag.Parse()

	if *outputFile == "" {
		glog.Fatal("--output flag cannot be empty")
	}

	c := crawl.New(
		crawl.WithConcurrency(1),
		crawl.WithQueue(crawl.NewQueue(1000)),
	)

	spider := &spider{c: c, results: make(chan *userAgent, 10000)}
	c.Register("list", spider.parseList)
	c.Register("user-agents", spider.parseUserAgents)

	ctx := context.Background()
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second*30))
	defer cancel()

	if err := c.Schedule(ctx, &crawl.Request{
		URL:       "http://www.useragentstring.com/pages/useragentstring.php",
		Callbacks: crawl.Callbacks("list"),
	}); err != nil {
		glog.Fatal(err)
	}

	glog.Info("Starting crawl")

	go func() {
		for err := range c.Errors() {
			glog.Infof("Crawl error: %v", err)
		}
	}()

	go c.Start()

	f, err := os.Create(*outputFile)
	if err != nil {
		glog.Fatal(err)
	}
	defer f.Close()

	var results []*userAgent
	for result := range spider.results {
		results = append(results, result)
	}

	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		glog.Fatal(err)
	}
	if _, err := f.Write(b); err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Done (%d user agents)", len(results))
}

type spider struct {
	c       crawl.Crawler
	results chan *userAgent
}

type userAgent struct {
	Type       string   `json:"type,omitempty"`
	Title      string   `json:"title,omitempty"`
	UserAgents []string `json:"user_agents,omitempty"`
}

func (spider *spider) parseList(ctx context.Context, resp *crawl.Response) error {
	defer spider.c.Close()

	var currentTitle string

	resp.Find("div#unterMenu a").Each(func(_ int, s *goquery.Selection) {
		c, _ := s.Attr("class")
		switch c {
		case "unterMenuTitel":
			currentTitle = strings.ToLower(s.Text())
		case "unterMenuName":
			ctx = metadata.NewContext(ctx, metadata.Pairs(
				"type", currentTitle,
				"title", s.Text(),
			))
			href, _ := s.Attr("href")
			spider.c.Execute(ctx, &crawl.Request{
				URL:       strings.TrimSpace(href),
				Referer:   resp.URL().String(),
				Callbacks: crawl.Callbacks("user-agents"),
			})
		}
	})
	close(spider.results)
	return nil
}

func (spider *spider) parseUserAgents(ctx context.Context, resp *crawl.Response) error {
	md, _ := metadata.FromContext(ctx)

	userAgentType := md["type"][0]
	userAgentTitle := md["title"][0]

	userAgents := resp.Find(`#liste li a`).Map(crawl.NodeText)

	glog.Info(resp.URL().String())
	glog.Infof("type=%q title=%q user-agents=%v", userAgentType, userAgentTitle, userAgents)

	spider.results <- &userAgent{
		Type:       userAgentType,
		Title:      userAgentTitle,
		UserAgents: userAgents,
	}

	return nil
}
