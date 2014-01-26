//
//  ⚠AWS-Notifies Status (RSS->Email)
//  Cloudwalk ⚙ CORE TEAM
//  dgv@cloudwalk.io
//

package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"

	"appengine"
	"appengine/mail"
	"appengine/memcache"
	"appengine/urlfetch"
)

// Subscribed services
var services = []string{
  "http://status.aws.amazon.com/rss/cloudsearch-us-east-1.rss",
  "http://status.aws.amazon.com/rss/ec2-us-east-1.rss",
}

// Check current status, if changed, notify!
func check(w http.ResponseWriter, r *http.Request) {
	for _, service := range services {
		if title, description := statusChange(r, service); title != "" {
			// Subscribe emails here
			sendStatus(r, title, description, "danielgvargas@gmail.com")
		}
	}
}

// Handle service in /check
func init() {
	http.HandleFunc("/check", check)
}

// AWS feed struct
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Chann   Channel  `xml:"channel"`
}

type Title struct {
	Title string `xml:"title"`
	Type  string `xml:"type,attr,omitempty"`
}

type Link struct {
	Rel   string `xml:"rel,attr,omitempty"`
	Href  string `xml:"href,attr,omitempty"`
	Type  string `xml:"type,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
}

type Channel struct {
	Titles    []Title `xml:"title"`
	Links     []Link  `xml:"link"`
	Language  string  `xml:"language"`
	PubDate   string  `xml:"pubDate"`
	Updated   string  `xml:"updated"`
	Generator string  `xml:"generator"`
	TTL       string  `xml:"ttl"`
	Items     []Item  `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
}

// Verify last pubDate of last entry in cache,
// if status had changes returns provide notification title/description
func statusChange(r *http.Request, feed_url string) (title, description string) {
	c := appengine.NewContext(r)
	client := urlfetch.Client(c)
	res, err := client.Get(feed_url)
	if err != nil {
		c.Errorf(err.Error())
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		c.Errorf(err.Error())
	}
	var rssNow RSS
	pubDate, err := memcache.Get(c, feed_url)
	if err != nil {
		if err == memcache.ErrCacheMiss {
			pubDate = &memcache.Item{Key: feed_url, Value: []byte("")}
		} else {
			c.Errorf(err.Error())
		}
	}
	err = xml.Unmarshal(body, &rssNow)
	if err != nil {
		c.Errorf(err.Error())
	}
	if !bytes.Equal(pubDate.Value, []byte(rssNow.Chann.Items[0].PubDate)) {
		pubDate.Value = []byte(rssNow.Chann.Items[0].PubDate)
		if err := memcache.Add(c, pubDate); err == memcache.ErrNotStored {
			err = memcache.Set(c, pubDate)
			if err != nil {
				c.Errorf(err.Error())
			}
		}
		return rssNow.Chann.Items[0].Title, rssNow.Chann.Items[0].Description
	}
	return "", ""
}

// Send notification message via GAE Mail API
func sendStatus(r *http.Request, subject, body string, to ...string) {
	c := appengine.NewContext(r)
	msg := &mail.Message{
		Sender:  "⚠ AWS-Notifies <aws.notifies@gmail.com>",
		To:      to,
		Subject: subject,
		Body:    fmt.Sprintf(notificationMessage, body),
	}
	if err := mail.Send(c, msg); err != nil {
		c.Errorf("Couldn't send email: %v", err)
	}
}

const notificationMessage = `

%s

http://status.aws.amazon.com

---
aws-notifies.go :: AWS Service Health Dashboard RSS Generator
CloudWalk ⚙ CORE TEAM
`
