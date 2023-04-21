package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nektro/go-util/ansi/style"
	"github.com/nektro/go-util/mbpp"
	"github.com/nektro/go-util/util"
	dbstorage "github.com/nektro/go.dbstorage"
	"github.com/spf13/pflag"
	"github.com/valyala/fastjson"
)

var (
	DoneDir   = "./data/"
	dbP       dbstorage.Database
	dbC       dbstorage.Database
	doComms   bool = false
	noPics    bool = false
	noDmDir   bool = true
	sortByTop bool = false
	wg             = new(sync.WaitGroup)
)
var (
	netClient = &http.Client{
		Timeout: time.Second * 5,
	}
)

func main() {
	flagSubr := pflag.StringArrayP("subreddit", "r", []string{}, "The name of a subreddit to archive. (ex. AskReddit, unixporn, CasualConversation, etc.)")
	flagUser := pflag.StringArrayP("user", "u", []string{}, "The name of a user to archive. (ex. spez, PoppinKREAM, Shitty_Watercolour, etc.)")
	flagDomn := pflag.StringArrayP("domain", "d", []string{}, "The host of a domain to archive.")
	flagSaveDir := pflag.String("save-dir", "", "Path to a directory to save to.")
	flagConcurr := pflag.Int("concurrency", 10, "Maximum number of simultaneous downloads.")
	pflag.BoolVar(&doComms, "do-comments", false, "Enable this flag to save post comments.")
	pflag.BoolVar(&sortByTop, "sort-top", false, "Enable this flag to download top posts instead of recent ones.")
	pflag.Parse()

	if sortByTop && doComms {
		// The way the original programmer structured this program makes it hard to fetch comments if sorting by top.
		// (they fetch /r/<sub>/comments/ and not comments per post)
		// And rewriting it would take too much effort.
		doComms = false
		util.Log("Fetching comments is disabled when sorting posts by top.")
	}

	if len(*flagSaveDir) > 0 {
		DoneDir = *flagSaveDir
	}
	DoneDir, _ = filepath.Abs(DoneDir)
	if !noDmDir {
		DoneDir += "/reddit.com"
	}
	os.MkdirAll(DoneDir, os.ModePerm)

	dbP = dbstorage.ConnectSqlite(DoneDir + "/posts.db")
	dbP.CreateTableStruct("posts", tPost{})

	dbC = dbstorage.ConnectSqlite(DoneDir + "/comments.db")
	dbC.CreateTableStruct("comments", tComment{})

	//

	util.RunOnClose(onClose)
	mbpp.Init(*flagConcurr)

	//

	items := [][2]string{}

	for _, item := range *flagSubr {
		items = append(items, [2]string{"r/%s", item})
	}
	for _, item := range *flagUser {
		items = append(items, [2]string{"u/%s/submitted", item})
	}
	for _, item := range *flagDomn {
		items = append(items, [2]string{"domain/%s", item})
	}

	//

	mbpp.CreateJob("reddit.com", func(bar *mbpp.BarProxy) {
		bar.AddToTotal(int64(len(items)))
		for _, item := range items {
			wg.Add(2)
			go fetchListing(fmt.Sprintf(item[0], item[1]), "", postListingCb)
			go fetchListing(fmt.Sprintf(item[0], item[1])+"/comments", "", commentListingCb)
			wg.Wait()
			bar.Increment(1)
		}
	})

	//

	time.Sleep(time.Second / 2)
	mbpp.Wait()
	onClose()
}

func onClose() {
	util.Log(mbpp.GetCompletionMessage())
}

func fetchListing(loc, after string, f func(string, string, *fastjson.Value) (bool, bool)) {
	next := ""
	s := strings.Split(loc, "/")
	t := s[0]
	name := s[1]

	jobname := style.FgRed + t + style.ResetFgColor + "/"
	jobname += style.FgCyan + name + style.ResetFgColor + " +"
	jobname += style.FgYellow + after + style.ResetFgColor
	mbpp.CreateJob(jobname, func(bar1 *mbpp.BarProxy) {
		if len(after) > 0 {
			after = "&after=" + after
		}

		var fetchURL string
		if sortByTop {
			fetchURL = "https://old.reddit.com/" + loc + "/top" + "/.json?show=all&t=all" + after
		} else {
			fetchURL = "https://old.reddit.com/" + loc + "/.json?show=all" + after
		}

		res, _ := fetch(http.MethodGet, fetchURL)
		bys, _ := io.ReadAll(res.Body)
		val, _ := fastjson.Parse(string(bys))

		next = string(val.GetStringBytes("data", "after"))

		ar := val.GetArray("data", "children")
		bar1.AddToTotal(int64(len(ar)))
		for _, item := range ar {
			end, skip := f(t, name, item)
			bar1.Increment(1)
			if end {
				next = ""
			}
			if skip {
				continue
			}
		}
	})
	if len(next) > 0 {
		wg.Add(1)
		fetchListing(loc, next, f)
	}
	wg.Done()
}
