package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/BlackEspresso/crawlbase"
	"github.com/BlackEspresso/htmlcheck"
	"github.com/PuerkitoBio/goquery"
	"github.com/robertkrimen/otto"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os/exec"
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
	http.HandleFunc("/test", testSite)
	http.HandleFunc("/api/crawl", apiCrawlRequest)
	http.HandleFunc("/api/dcrawl", apiDynamicCrawlRequest)
	http.HandleFunc("/api/addTag", apiAddTag)
	http.HandleFunc("/api/runScript", apiRunScript)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func staticSites(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadFile("index.html")
	logFatal(err)
	w.Write(b)
}

func testSite(w http.ResponseWriter, r *http.Request) {
	inpage := r.URL.Query().Get("inpage")
	inscript := r.URL.Query().Get("inscript")
	
	w.Write([]byte("<html>" + inpage+"<script>var m = '"+inscript+
		"';document.write(m);document.cookie='test='+m</script></html>"))
}

func apiRunScript(w http.ResponseWriter, r *http.Request) {
	script := r.URL.Query().Get("script")
	vm := otto.New()
	v, err := vm.Run(script)
	if err != nil {
		log.Println(err)
		return
	}
	val, _ := v.ToString()
	b, err := json.Marshal(val)
	logFatal(err)
	w.Write(b)
}

func apiDynamicCrawlRequest(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	page := crawlDynamic(url)

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

	if tName == "" {
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

	t, _ := findTag(tags, tName)

	log.Println(tName, aName, tag.Attrs)
	log.Println(t)
	writeTagsToFile(tags)

	w.Write([]byte("OK"))
}

func findString(arrs []string, name string) bool {
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
			return &tags[i], true
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

type PhJsPage struct {
	Body     string
	JSwrites []string
	JSevals []string
	JStimeouts []string
	Cookies []crawlbase.Cookie
	Requests []string
}

func crawlDynamic(urlStr string) *crawlbase.Page {
	cmd := exec.Command("phantomjs", "getsource.js", urlStr)
	//cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	cmd.Stdout = &out
	timeStart := time.Now()
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	timeDur := time.Now().Sub(timeStart)

	page := crawlbase.Page{}
	PhJsPage := PhJsPage{}
	err = json.Unmarshal(out.Bytes(),&PhJsPage)
	logFatal(err)
	page.Body = PhJsPage.Body

	ioreader := bytes.NewReader([]byte(PhJsPage.Body))
	doc, err := goquery.NewDocumentFromReader(ioreader)
	logFatal(err)

	baseUrl, err := url.Parse(urlStr)
	logFatal(err)

	page.Hrefs = crawlbase.GetHrefs(doc, baseUrl)
	page.Forms = crawlbase.GetFormUrls(doc, baseUrl)
	page.Ressources = crawlbase.GetRessources(doc, baseUrl)

	page.CrawlTime = int(time.Now().Unix())
	page.Url = urlStr
	page.RespCode = 200
	page.RespDuration = int(timeDur.Seconds() * 1000)
	page.Uid = crawlbase.ToSha256(urlStr)
	page.Cookies = PhJsPage.Cookies;

	jsinfos := []crawlbase.JSInfo{}
	for _,v:=range PhJsPage.JSwrites{
		info := crawlbase.JSInfo{"document.write",v}
		jsinfos = append(jsinfos,info)
	}
	for _,v:=range PhJsPage.JSevals{
		info := crawlbase.JSInfo{"eval",v}
		jsinfos = append(jsinfos,info)
	}
	for _,v:=range PhJsPage.JStimeouts{
		info := crawlbase.JSInfo{"setTimeout",v}
		jsinfos = append(jsinfos,info)
	}
	for _,v:=range PhJsPage.Requests{
		info := crawlbase.Ressource{v,"","",""}
		page.Ressources = append(page.Ressources,info)
	}
	
	page.JSInfo = jsinfos

	return &page
}

func crawl(urlStr string) *crawlbase.Page {
	timeStart := time.Now()
	client := http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", urlStr, nil)
	logFatal(err)

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
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
	page.Hrefs = crawlbase.GetHrefs(doc, baseUrl)
	page.Forms = crawlbase.GetFormUrls(doc, baseUrl)
	page.Ressources = crawlbase.GetRessources(doc, baseUrl)

	page.CrawlTime = int(time.Now().Unix())
	page.Url = urlStr
	page.RespCode = res.StatusCode
	page.RespDuration = int(timeDur.Seconds() * 1000)
	page.Uid = crawlbase.ToSha256(urlStr)
	page.Body = string(body)
	return &page
}

func logFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
