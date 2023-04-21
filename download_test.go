package main

import "testing"

type linkMap struct {
	posted    string
	extracted string
}

var testsTable = []linkMap{
	{"https://imgur.com/a/eEFau6M", "https://i.imgur.com/wrMEBlUh.jpg"},
	{"https://i.imgur.com/URffwD5.gifv", "https://i.imgur.com/URffwD5.mp4"},
	{"https://imgur.com/gallery/2dZx1Np", "https://i.imgur.com/V3vlnns.mp4"},
	{"https://imgur.com/a/iigM45Z", "https://i.imgur.com/9hdIjyZ.mp4"},
	{"https://imgur.com/9hdIjyZ", "https://i.imgur.com/9hdIjyZ.mp4"},
	{"https://i.imgur.com/K0tIsTl.png", "https://i.imgur.com/K0tIsTl.png"},
	{"https://gfycat.com/whiteoptimisticgrackle-work",
		"https://thumbs.gfycat.com/WhiteOptimisticGrackle-mobile.mp4"},
}

func TestExtractDownloadLink(t *testing.T) {
	for _, test := range testsTable {
		extractedMedia, err := extractDownloadLink(test.posted)
		if err != nil {
			t.Errorf("Unexpected Error: %v", err)
		}
		if extractedMedia[0].downloadLink != test.extracted {
			t.Errorf("Extraction Error, Extracted: %s and Expected: %s ", extractedMedia[0].downloadLink, test.extracted)
		}
	}
}
