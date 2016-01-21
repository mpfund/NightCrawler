var webserver = require('webserver');
var webPage = require('webpage');

var server = webserver.create();

var service = server.listen(8081, function (request, response) {
    response.statusCode = 200;
    var q = queriesFromUrl(request.url);
    if (q.url == null){
        response.close();
        return;
    }

    var page = webPage.create();
    var url = decodeURIComponent(q.url)

    getSite(page, url, q.cookie,function(retObj){
        response.write(JSON.stringify(retObj));
        response.close();
        page.close();
    });
});

function queriesFromUrl(url) {
    var urlSplitted = url.split('?');
    if (urlSplitted.length <= 1)
        return {};

    var obj = {};
    var querySplitted = urlSplitted[1].split('&');

    for (var x = 0; x < querySplitted.length; x++) {
        var kv = querySplitted[x].split('=')
        obj[kv[0]] = kv[1];
    }
    return obj;
}

function getSite(page, url,cookie, done) {
    page.onConsoleMessage = function (msg) { };
    page.onError = function () {};

    var requests = [];

    page.onResourceRequested = function (requestData, networkRequest) {
        requests.push(requestData.url)
    };

    page.onInitialized = function () {
        page.evaluate(function () {
            window.document.__write = document.write;
            window.document.__writes = [];
            window.__evals = [];
            window.__timeouts = [];
            window.document.write = function (k) {
                window.document.__writes.push(k);
                window.document.__write(k);
            }
            window.document.__writeln = document.writeln;
            window.document.writeln = function (k) {
                window.document.__writes.push(k);
                window.document.__writeln(k);
            }
            window.__eval = window.eval;
            window.eval = function (k) {
                window.__evals.push(k);
                return window.__eval(k);
            }
            /*
             window.__setTimeout = window.setTimeout;
             window.setTimeout = function(a,b,c){
             if(typeof(a)===typeof('')){window.__timeouts.push(totext(a).substr(0,40);}
             return window.__setTimeout(a,b,c);
             }*/
        });
    };


    if(cookie!=null) {

        page.addCookie({
            domain: ".google.de",
            expires: (new Date()).getTime() + (1000 * 60 * 60),
            httponly: false,
            name: "test",
            path: "/",
            secure: false,
            value: "xxxxx"
        });
    }

    page.open(url, function (status) {
        setTimeout(function () {report(status)}, 1000)
    });

    function report(status) {
        var writes = page.evaluate(function () {
            return window.document.__writes;
        })
        var evals = page.evaluate(function () {
            return window.__evals;
        })
        var timeouts = page.evaluate(function () {
            return window.__timeouts;
        })

        var ret = {
            Body: null,
            JSwrites: null,
            JSwrites: null,
            JSevals: null,
            JStimeoutes: [],
            Cookies: null,
            Requests: null
        }

        if (status == 'success') {
            ret = {
                Body: page.content,
                JSwrites: writes,
                JSevals: evals,
                JStimeouts: [],
                Cookies: page.cookies,
                Requests: requests
            };
            done(ret);
            return
        }
        done(ret);
    }
}

function totext(k) {
    if (k === null)
        return null;
    return k.toString();
}
