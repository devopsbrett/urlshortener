package store

import (
	"encoding/xml"
	"net/url"
	"time"
)

type URL struct {
	XMLName   xml.Name  `json:"-" xml:"shorten"`
	URL       string    `json:"url" xml:"URL"`
	ID        string    `json:"id,omitempty" xml:"id,attr,omitempty"`
	ShortURL  string    `json:"short_url,omitempty" xml:"ShortURL,omitempty"`
	CreatorIP string    `json:"creator_ip,omitempty" xml:"CreatorIP,omitempty"`
	DateAdded time.Time `json:"date_added,omitempty" xml:"DateAdded,omitempty"`
	Visits    int       `json:"times_visited" xml:"TimesVisited"`
}

func NewURL(s Store, urlStr string, ip string) (URL, error) {
	var urlObj URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return urlObj, err
	}
	urlObj.URL = u.String()
	urlObj.CreatorIP = ip

	err = s.Store(&urlObj)
	return urlObj, err
}
