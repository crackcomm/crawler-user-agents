package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/mssola/user_agent"
)

var inputFile = flag.String("input", "crawler-user-agents_2.json", "input file")

func init() {
	flag.Set("logtostderr", "true")
}

type userAgent struct {
	Type       string   `json:"type,omitempty"`
	Title      string   `json:"title,omitempty"`
	UserAgents []string `json:"user_agents,omitempty"`
}

func main() {
	defer glog.Flush()
	flag.Parse()

	if *inputFile == "" {
		glog.Fatal("--input flag cannot be empty")
	}

	b, err := ioutil.ReadFile(*inputFile)
	if err != nil {
		glog.Fatal(err)
	}

	var userAgents []*userAgent
	err = json.Unmarshal(b, &userAgents)
	if err != nil {
		glog.Fatal(err)
	}

	var all, success int
	for _, client := range userAgents {
		all++
		if testClient(client) {
			success++
		}
	}
	glog.Infof("done %d/%d %.2f%%", success, all, (float64(success)/float64(all))*100.0)
}

func testClient(client *userAgent) (success bool) {
	var crawler bool
	switch client.Type {
	case "crawlers", "browsers":
		crawler = client.Type == "crawlers"
		break
	default:
		return
	}

	for _, uastr := range client.UserAgents {
		ua := user_agent.New(uastr)
		if crawler && !ua.Bot() {
			glog.Warningf("%s not recognized %q", client.Type, uastr)
		} else if !crawler && ua.Bot() {
			glog.Warningf("%s recognized as bot %q", client.Type, uastr)
		} else {
			return true
		}
	}
	return
}
