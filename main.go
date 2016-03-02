package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"./httpmitm"
	"./servertasks"
	"github.com/BlackEspresso/crawlbase"
	"github.com/BlackEspresso/htmlcheck"

	"gopkg.in/mgo.v2"
)

var session *mgo.Session
var db *mgo.Database

var userAgentHeader string = "Mozilla/5.0 (Windows NT 6.3; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.106 Safari/537.36"

func main() {

	r := gin.Default()

	r.Static("/static", "./static")

	//r.GET("/login", login)
	r.GET("/test", testSite)

	r.GET("/api/getsiteinfo", apiGetSiteInfo)
	r.GET("/api/addTag", apiAddTag)
	r.GET("/api/proxyrequests", apiProxyRequests)
	//r.GET("/tasks", servertasks.GenHandler(gHandler))

	var err error
	session, err = mgo.Dial("localhost")
	db = session.DB("nightcrawler")

	if err != nil {
		panic(err)
	}

	defer session.Close()

	servertasks.Start()

	go func() {
		simpleProxyHandler := http.HandlerFunc(httpmitm.GenSimpleHandlerFunc(req, resp))
		http.ListenAndServe(":8081", simpleProxyHandler)
	}()

	r.Run(":8080")
}

func req(r *http.Request) {

}

func resp(r *http.Request, req *http.Response, dur time.Duration) {
	cw := crawlbase.NewCrawler()
	page := cw.PageFromResponse(r, req, dur)
	c := db.C("requests")
	c.Insert(page)
}

func testSite(g *gin.Context) {
	inpage, _ := g.GetQuery("inpage")
	inscript, _ := g.GetQuery("inscript")
	ineval, _ := g.GetQuery("ineval")

	g.String(200, "<html>"+inpage+"<script>var m = '"+inscript+
		"';document.write(m);document.cookie='test='+m</script>"+
		"<script>eval("+ineval+")</script>"+
		"<script>document.write(decodeURIComponent(document.location.hash))</script>"+
		"</html>")
}

func apiProxyRequests(g *gin.Context) {
	c := db.C("requests")
	var pages []crawlbase.Page
	c.Find(nil).All(&pages)
	g.JSON(200, pages)
}

func apiGetSiteInfo(g *gin.Context) {
	url, ok := g.GetQuery("url")
	if !ok {
		g.String(500, "missing parameter url")
		return
	}
	_, inclBody := g.GetQuery("body")

	cw := crawlbase.NewCrawler()
	cw.Header.Add("User-Agent", userAgentHeader)
	tags, _ := crawlbase.LoadTagsFromFile("tags.json")
	cw.Validator.AddValidTags(tags)

	page, err := cw.GetPage(url, "GET")

	if err != nil {
		g.String(500, err.Error())
	}

	// first check

	if !inclBody {
		page.RespInfo.Body = ""
	}

	g.JSON(200, page)
}

func apiAddTag(g *gin.Context) {
	if g.Request.Method != "POST" {
		g.String(404, "not a get")
		return
	}

	tName, _ := g.GetQuery("TagName")
	aName, _ := g.GetQuery("AttributeName")

	if tName == "" {
		g.String(200, "TagName is empty")
		return
	}

	tags, _ := crawlbase.LoadTagsFromFile("tags.json")
	tag, ok := findTag(tags, tName)
	if !ok {
		tag = &htmlcheck.ValidTag{tName, []string{}, "", false}
		tags = append(tags, tag)
	}

	ok = findString(tag.Attrs, aName)
	if !ok {
		tag.Attrs = append(tag.Attrs, aName)
	}

	//t, _ := findTag(tags, tName)
	crawlbase.WriteTagsToFile(tags, "tags.json")

	g.String(200, "ok")
}

func findString(arrs []string, name string) bool {
	for _, v := range arrs {
		if v == name {
			return true
		}
	}
	return false
}

func findTag(tags []*htmlcheck.ValidTag, tagName string) (*htmlcheck.ValidTag, bool) {
	for i, v := range tags {
		if v.Name == tagName {
			return tags[i], true
		}
	}
	return nil, false
}

func logFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
