package main

import (
	"errors"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/nektro/go-util/mbpp"
)

func fetch(method, urlS string) (*http.Response, error) {
	req, _ := http.NewRequest(method, urlS, nil)
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36")
	res, _ := http.DefaultClient.Do(req)
	return res, nil
}

type media struct {
	downloadLink   string
	filenameSuffix string
}

func downloadPost(t, name string, id string, urlS string, dir string, title string) {
	if noPics {
		return
	}

	links, err := extractDownloadLink(urlS)
	if err != nil {
		return
	}

	if len(links) > 0 {
		os.MkdirAll(dir, os.ModePerm)

		for _, item := range links {
			go mbpp.CreateDownloadJob(item.downloadLink, dir+"/"+sanitizeFileName(title)+"_"+item.filenameSuffix, nil)
		}
	}
}

func extractDownloadLink(urlS string) ([]media, error) {
	links := []media{}

	urlO, err := url.Parse(urlS)
	if err != nil {
		return links, err
	}

	res, err := netClient.Head(urlS)
	if err != nil {
		return links, err
	}

	contentType := res.Header.Get("content-type")
	var isContentHTML bool
	if strings.Contains(contentType, "text/html") {
		isContentHTML = true
	} else {
		isContentHTML = false
	}

	if urlO.Host == "old.reddit.com" {
		return links, errors.New("not an image link")
	}

	if (urlO.Host == "i.redd.it" || urlO.Host == "i.imgur.com") && !isContentHTML {
		links = append(links, media{urlS, urlO.Host + "_" + urlO.Path[1:]})
	}

	if strings.Contains(urlO.Host, "imgur") && isContentHTML {
		ext := filepath.Ext(urlO.RequestURI())
		urlWithoutExt := strings.TrimSuffix(urlS, ext)

		// If its a gifv then we can download as an mp4 instead
		if ext == ".gifv" {
			links = append(links, media{urlWithoutExt + ".mp4", randomStringGen(5) + "_" + urlO.Host + ".mp4"})
		} else {
			res, err := http.Get(urlS)
			if err != nil {
				return links, err
			}

			doc, err := goquery.NewDocumentFromReader(res.Body)
			if err != nil {
				return links, err
			}

			videoURL, exists := doc.Find("meta[property='og:video']").Attr("content")
			if exists {
				links = append(links, media{videoURL, randomStringGen(5) + "_" + urlO.Host + ".mp4"})
			}

			imgURL, exists := doc.Find("meta[name='twitter:image']").Attr("content")
			if exists {
				links = append(links, media{imgURL, randomStringGen(5) + "_" + urlO.Host + ".jpg"})
			}
		}
	}

	if strings.Contains(urlO.Host, "giphy") {
		if contentType == "image/gif" {
			pid := strings.Split(urlS, "/")[2]
			links = append(links, media{urlS, urlO.Host + "_" + pid + ".gif"})
		} else if isContentHTML {
			res, err := fetch(http.MethodGet, urlS)
			if err != nil {
				return links, err
			}

			doc, err := goquery.NewDocumentFromReader(res.Body)
			if err != nil {
				return links, err
			}

			videoURL, exists := doc.Find("meta[property='og:video']").Attr("content")
			if exists {
				links = append(links, media{videoURL, randomStringGen(5) + "_" + urlO.Host + ".mp4"})
			}
		}
	}

	if (urlO.Host == "gfycat.com" || urlO.Host == "www.redgifs.com") && isContentHTML {
		res, err := fetch(http.MethodGet, urlS)
		if err != nil {
			return links, err
		}

		doc, err := goquery.NewDocumentFromReader(res.Body)
		if err != nil {
			return links, err
		}

		videoURL, exists := doc.Find("meta[property='og:video']").Attr("content")
		if exists {
			links = append(links, media{videoURL, randomStringGen(5) + "_" + urlO.Host + ".mp4"})
		}
	}

	if !isContentHTML && len(links) > 0 {
		fn := strings.TrimPrefix(urlO.Path, filepath.Dir(urlO.Path))
		links = append(links, media{urlS, urlO.Host + "_" + fn})
	}

	return links, nil
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

func randomStringGen(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
