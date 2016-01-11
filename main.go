package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/BlackEspresso/crawlbase"
	"github.com/BlackEspresso/htmlcheck"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

type Response struct {
	Page       *crawlbase.Page
	HtmlErrors []*htmlcheck.ValidationError
}

func main() {
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path[1:])
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.HandleFunc("/", staticSites)
	http.HandleFunc("/api/crawl", apiCrawlRequest)
	http.HandleFunc("/api/addTag", apiAddTag)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func staticSites(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadFile("index.html")
	logFatal(err)
	w.Write(b)
}

func apiCrawlRequest(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	page := crawl(url)
	validater := htmlcheck.Validator{}
	tags := loadTagsFromFile()
	validater.AddValidTags(tags)
	// first check

	htmlErrors := validater.ValidateHtmlString(page.Body)
	resp := Response{page, htmlErrors}

	b, err := json.Marshal(resp)
	logFatal(err)
	w.Write(b)
}

func apiAddTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	r.ParseForm()
	tName := r.Form.Get("TagName")
	aName := r.Form.Get("AttributeName")
	
	if(tName==""){
		w.Write([]byte("TagName is empty"))
		return
	}

	tags := loadTagsFromFile()
	tag, ok := findTag(tags, tName)
	if !ok {
		tag = &htmlcheck.ValidTag{tName, []string{}, false}
		tags = append(tags, *tag)
	}
	ok = findString(tag.Attrs, aName)
	if !ok {
		tag.Attrs = append(tag.Attrs, aName)
	}
	
	t,_:=findTag(tags,tName)
	
	log.Println(tName,aName,tag.Attrs)
	log.Println(t)
	writeTagsToFile(tags)

	w.Write([]byte("OK"))
}

func findString(arrs []string, name string) (bool) {
	for _, v := range arrs {
		if v == name {
			return true
		}
	}
	return false
}

func findTag(tags []htmlcheck.ValidTag, tagName string) (*htmlcheck.ValidTag, bool) {
	for i, v := range tags {
		if v.Name == tagName {
			return &tags[i] , true
		}
	}
	return nil, false
}

func writeTagsToFile(tags []htmlcheck.ValidTag) {
	b, err := json.Marshal(tags)
	logFatal(err)
	ioutil.WriteFile("tags.json", b, 755)
}

func loadTagsFromFile() []htmlcheck.ValidTag {
	content, err := ioutil.ReadFile("tags.json")
	logFatal(err)

	var validTags []htmlcheck.ValidTag
	err = json.Unmarshal(content, &validTags)
	logFatal(err)

	return validTags
}

func crawl(urlStr string) *crawlbase.Page {
	timeStart := time.Now()
	client := http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", urlStr, nil)
	logFatal(err)

	req.Header.Set("Accept","text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.3; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.106 Safari/537.36")

	res, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return nil
	}
	timeDur := time.Now().Sub(timeStart)

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	logFatal(err)

	fmt.Println("crawling " + urlStr)
	ioreader := bytes.NewReader(body)
	doc, err := goquery.NewDocumentFromReader(ioreader)
	logFatal(err)

	baseUrl, err := url.Parse(urlStr)
	logFatal(err)
	
	page := crawlbase.Page{}
	page.Hrefs = getHrefs(doc,baseUrl)
	page.Forms = getFormUrls(doc,baseUrl)
	page.Links = getLinks(doc,baseUrl)
	
	page.CrawlTime = int(time.Now().Unix())
	page.Url = urlStr
	page.RespCode = res.StatusCode
	page.RespDuration = int(timeDur.Seconds() * 1000)
	page.Uid = toSha256(urlStr)
	page.Body = string(body)
	return &page
}

func getLinks(doc *goquery.Document,baseUrl *url.URL)[]crawlbase.Link{
	links := []crawlbase.Link{}
	doc.Find("link").Each(func(i int, s *goquery.Selection) {
		link := crawlbase.Link{}
		href, exists := s.Attr("href")
		if exists{
			link.Url = href;
		}
		linkType, exists := s.Attr("type")
		if exists{
			link.Type = linkType;
		}
		links = append(links,link)
	})
	return links
}

func getHrefs(doc *goquery.Document,baseUrl *url.URL)[]string{
	hrefs := []string{}
	hrefsTest := map[string]bool{}
	
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			var fullUrl = toAbsUrl(baseUrl, href)
			_, isAlreadyAdded := hrefsTest[fullUrl]
			if !isAlreadyAdded {
				hrefsTest[fullUrl] = true
				hrefs = append(hrefs, fullUrl)
			}
		}
	})
	return hrefs
}

func getFormUrls(doc *goquery.Document,baseUrl *url.URL)[]crawlbase.Form{
	forms := []crawlbase.Form{}
	
	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		form := crawlbase.Form{}
		href, exists := s.Attr("action")
		if exists{
			form.Url = href;
		}
		method, exists := s.Attr("method")
		if exists{
			form.Method = method
		}
		form.Inputs = []crawlbase.FormInput{}
		s.Find("input").Each(func(i int, s *goquery.Selection){
			input := crawlbase.FormInput{}
			name, exists := s.Attr("name")
			if exists{
				input.Name = name
			}
			value, exists := s.Attr("value")
			if exists{
				input.Value = value
			}
			form.Inputs = append(form.Inputs,input)
		})
		
		forms = append(forms,form)
	})
	return forms
}

func toSha256(message string) string {
	h := sha256.New()
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func toAbsUrl(baseurl *url.URL, weburl string) string {
	relurl, err := url.Parse(weburl)
	if err != nil {
		return ""
	}
	absurl := baseurl.ResolveReference(relurl)
	return absurl.String()
}

func logFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
