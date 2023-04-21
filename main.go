package main

import (
	"fmt"
	"io"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
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

func fetch(method, urlS string) (*http.Response, error) {
	req, _ := http.NewRequest(method, urlS, nil)
	req.Header.Add("user-agent", "linux:eu.the-eye.reddit-dl:v1.0.0 (by /u/nektro)")
	res, _ := http.DefaultClient.Do(req)
	return res, nil
}

func findExtension(urlS string) string {
	res, _ := fetch(http.MethodHead, urlS)
	ext, _ := mime.ExtensionsByType(res.Header.Get("content-type"))
	return ext[0]
}

func downloadPost(t, name string, id string, urlS string, dir string, title string) {
	title = strings.Replace(title, " ", "_", -1)

	if noPics {
		return
	}

	urlO, err := url.Parse(urlS)
	if err != nil {
		return
	}

	res, err := netClient.Head(urlS)
	if err != nil {
		return
	}

	links := [][2]string{}
	ct := res.Header.Get("content-type")

	if urlO.Host == "old.reddit.com" {
		return
	}
	if urlO.Host == "i.redd.it" || urlO.Host == "i.imgur.com" || (urlO.Host == "imgur.com" && !strings.Contains(ct, "text/html")) {
		links = append(links, [2]string{urlS, urlO.Host + "_" + urlO.Path[1:]})
	}
	if urlO.Host == "imgur.com" && strings.Contains(ct, "text/html") {
		res, _ := fetch(http.MethodGet, urlS)
		doc, _ := goquery.NewDocumentFromResponse(res)
		doc.Find(".post-images .post-image-container").Each(func(_ int, el *goquery.Selection) {
			pid, _ := el.Attr("id")
			ext := findExtension("https://i.imgur.com/" + pid + ".png")
			links = append(links, [2]string{"https://i.imgur.com/" + pid + ext, urlO.Host + "_" + pid + ext})
		})
	}
	if urlO.Host == "media.giphy.com" && ct == "image/gif" {
		pid := strings.Split(urlS, "/")[2]
		links = append(links, [2]string{urlS, urlO.Host + "_" + pid + ".gif"})
	}
	if (urlO.Host == "gfycat.com" || urlO.Host == "www.redgifs.com") && strings.Contains(ct, "text/html") {
		res, _ := fetch(http.MethodGet, urlS)
		webBody, err := io.ReadAll(res.Body)
		if err == nil {
			finalLink := getStringInBetween(string(webBody), `property="og:video" content="`, `">`)
			links = append(links, [2]string{finalLink, randomStringGen(5) + "_" + urlO.Host + ".mp4"})
		}

	}
	if !strings.Contains(ct, "text/html") {
		fn := strings.TrimPrefix(urlO.Path, filepath.Dir(urlO.Path))
		links = append(links, [2]string{urlS, urlO.Host + "_" + fn})
	}
	if len(links) > 0 {
		os.MkdirAll(dir, os.ModePerm)

		for _, item := range links {
			go mbpp.CreateDownloadJob(item[0], dir+"/"+sanitizeFileName(title)+"_"+item[1], nil)
		}
	}
}

// Utilities
func sanitizeFileName(input string) string {
	replacer := strings.NewReplacer(
		",", "", ".", "", "-", "", ":", "", "!", "", "?", "", "[", "", "]", "", "(", "", ")", "", "/", "",
	)
	input = strings.TrimSpace(input)
	input = strings.ReplaceAll(input, " ", "_")
	input = replacer.Replace(input)
	return input
}

func getStringInBetween(str string, start string, end string) (result string) {
	s := strings.Index(str, start)
	if s == -1 {
		return
	}
	s += len(start)
	e := strings.Index(str[s:], end)
	if e == -1 {
		return
	}
	return str[s : s+e]
}

func randomStringGen(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
