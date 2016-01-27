package main

import (
	"./httpmitm"
	"./servertasks"
	"bytes"
	"encoding/json"
	"github.com/BlackEspresso/crawlbase"
	"github.com/BlackEspresso/htmlcheck"
	"github.com/PuerkitoBio/goquery"
	"github.com/robertkrimen/otto"
	"gopkg.in/mgo.v2"
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

var session *mgo.Session

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
	http.HandleFunc("/api/scripting", apiRunScript)
	http.HandleFunc("/api/proxyrequests", apiProxyRequests)
	http.HandleFunc("/tasks", servertasks.GenHandler(gHandler))
	var err error
	session, err = mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer session.Close()

	servertasks.Start()

	go func() {
		simpleProxyHandler := http.HandlerFunc(httpmitm.GenSimpleHandlerFunc(req, resp))
		http.ListenAndServe(":8081", simpleProxyHandler)
	}()
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func gHandler(t *servertasks.TaskBlock, r *http.Request) {
	js := r.FormValue("jsFunc")
	t.Func = func(task *servertasks.TaskBlock) {
		vm := otto.New()
		k, _ := vm.Run(js)
		task.Done = true

		txt, _ := k.ToString()
		task.SuccessText = txt
	}
}

func req(r *http.Request) {

}

func resp(r *http.Request, req *http.Response, dur time.Duration) {
	page := PageFromResponse(r, req, dur)
	c := session.DB("checkSite").C("requests")
	c.Insert(page)
}

func staticSites(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadFile("index.html")
	logFatal(err)
	w.Write(b)
}

func testSite(w http.ResponseWriter, r *http.Request) {
	inpage := r.URL.Query().Get("inpage")
	inscript := r.URL.Query().Get("inscript")
	ineval := r.URL.Query().Get("ineval")

	w.Write([]byte("<html>" + inpage + "<script>var m = '" + inscript +
		"';document.write(m);document.cookie='test='+m</script>" +
		"<script>eval(" + ineval + ")</script>" +
		"<script>document.write(decodeURIComponent(document.location.hash))</script>" +
		"</html>"))
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

func apiProxyRequests(w http.ResponseWriter, r *http.Request) {
	c := session.DB("checkSite").C("requests")
	var pages []crawlbase.Page
	c.Find(nil).All(&pages)
	k, _ := json.Marshal(pages)
	w.Write(k)
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
	Body       string
	JSwrites   []string
	JSevals    []string
	JStimeouts []string
	Cookies    []crawlbase.Cookie
	Requests   []string
}

func PageFromResponse(req *http.Request, res *http.Response, timeDur time.Duration) *crawlbase.Page {
	page := crawlbase.Page{}
	body, err := ioutil.ReadAll(res.Body)

	if err == nil {
		page.Body = string(body)
		ioreader := bytes.NewReader(body)
		doc, err := goquery.NewDocumentFromReader(ioreader)
		if err == nil {
			page.Hrefs = crawlbase.GetHrefs(doc, req.URL)
			page.Forms = crawlbase.GetFormUrls(doc, req.URL)
			page.Ressources = crawlbase.GetRessources(doc, req.URL)
		}
	}

	page.CrawlTime = int(time.Now().Unix())
	page.URL = req.URL.String()
	page.RequestURI = req.RequestURI
	page.Uid = crawlbase.ToSha256(page.URL)
	page.RespCode = res.StatusCode
	page.RespDuration = int(timeDur.Seconds() * 1000)
	return &page
}

func crawlDynamic(urlStr string) *crawlbase.Page {
	timeStart := time.Now()
	res, err := http.Get("http://localhost:8079/?url=" + url.QueryEscape(urlStr))
	logFatal(err)
	timeDur := time.Now().Sub(timeStart)

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	logFatal(err)

	PhJsPage := PhJsPage{}

	err = json.Unmarshal(body, &PhJsPage)
	logFatal(err)

	page := crawlbase.Page{}
	page.Body = PhJsPage.Body

	ioreader := bytes.NewReader([]byte(page.Body))
	doc, err := goquery.NewDocumentFromReader(ioreader)
	logFatal(err)

	baseUrl, err := url.Parse(urlStr)
	logFatal(err)

	page.Hrefs = crawlbase.GetHrefs(doc, baseUrl)
	page.Forms = crawlbase.GetFormUrls(doc, baseUrl)
	page.Ressources = crawlbase.GetRessources(doc, baseUrl)

	page.CrawlTime = int(time.Now().Unix())
	page.URL = urlStr
	page.RespCode = 200
	page.RespDuration = int(timeDur.Seconds() * 1000)
	page.Uid = crawlbase.ToSha256(urlStr)
	page.Cookies = PhJsPage.Cookies

	jsinfos := []crawlbase.JSInfo{}
	for _, v := range PhJsPage.JSwrites {
		info := crawlbase.JSInfo{"document.write", v}
		jsinfos = append(jsinfos, info)
	}
	for _, v := range PhJsPage.JSevals {
		info := crawlbase.JSInfo{"eval", v}
		jsinfos = append(jsinfos, info)
	}
	for _, v := range PhJsPage.JStimeouts {
		info := crawlbase.JSInfo{"setTimeout", v}
		jsinfos = append(jsinfos, info)
	}
	for _, v := range PhJsPage.Requests {
		info := crawlbase.Ressource{v, "", "", ""}
		page.Requests = append(page.Requests, info)
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

	page := PageFromResponse(req, res, timeDur)
	return page
}

func logFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
