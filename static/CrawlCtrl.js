/// <reference path="./TypeFiles/angular.d.ts"/>
angular.module('crawlApp', ['ui.bootstrap'])
    .controller('CrawlCtrl', function ($scope, $http, $sce) {
    $scope.page = null;
    $scope.htmlErrors = [];
    $scope.showErrorTab = true;
    $scope.scans = [];
    $scope.htmlValidator = {};
    $scope.scanner = {};
    $scope.oldUrlQuery = null;
    loadUserSettings();
    $scope.setUrl = function (l) {
        $scope.inputUrl = l;
    };
    $scope.addToScan = function (l) {
        if ($scope.scans.indexOf(l) == -1)
            $scope.scans.push(angular.copy(l));
    };
    $scope.generateScanAllParams = function (qp) {
        var vecs = $scope.scanner.scanVectorsArr;
        var urlinlist = [];
        for (var x = 0; x < qp.order.length; x++) {
            var k = qp.order[x];
            for (var y = 0; y < vecs.length; y++) {
                var m = angular.copy(qp);
                m.queries[k] = vecs[y];
                m.fullurl = buildUrlFromQueryParams(m);
                if (urlinlist.indexOf(m.fullurl) != -1)
                    continue;
                urlinlist.push(m.fullurl);
                $scope.scans.push(m);
            }
        }
    };
    $scope.clearScanList = function () {
        $scope.scans = [];
    };
    $scope.crawlScanList = function (l) {
        var scans = $scope.scans;
        var hvr = $scope.htmlValidator;
        hvr.hasDanger = false;
        var tags = hvr.dangerTags.split(',');
        var attrs = hvr.dangerAttributes.split(',');
        for (var x = 0; x < scans.length; x++) {
            var scan = scans[x];
            var url = scan.fullurl;
            crawl(url, scan).then(function (arr) {
                var resp = arr[0];
                var scan = arr[1];
                scan.resp = resp.data;
                scan.resp.hasDanger = checkDangerTags(scan.resp.HtmlErrors, tags, attrs);
            });
        }
    };
    $scope.crawlPage = function () {
        // clear container
        saveUserSettings();
        setStatus('Loading...');
        // request
        var url = $scope.inputUrl;
        if ($scope.encodeUrl)
            url = encodeURI(url);
        crawl(url, null).then(onNewCrawl);
    };
    function crawl(url, k) {
        return $http.get('/api/dcrawl?url=' + encodeURIComponent(url))
            .then(function (resp) { return [resp, k]; });
    }
    function onNewCrawl(arr) {
        var resp = arr[0];
        $scope.page = resp.data;
        applyFilter();
        setStatus('');
        setIFrame($scope.page.Page.Body);
        var arr = $scope.page.HtmlErrors;
        var hvr = $scope.htmlValidator;
        hvr.hasDanger = false;
        var tags = hvr.dangerTags.split(',');
        var attrs = hvr.dangerAttributes.split(',');
        hvr.hasDanger = checkDangerTags(arr, tags, attrs);
    }
    function checkDangerTags(htmlErrors, tags, attrs) {
        var hasDanger = false;
        for (var x = 0; x < htmlErrors.length; x++) {
            var entry = htmlErrors[x];
            var isDangerTag = tags.indexOf(entry.TagName) != -1;
            var isDangerAttribute = attrs.indexOf(entry.AttributeName) != -1;
            entry.isDanger = isDangerTag || isDangerAttribute;
            hasDanger = hasDanger || entry.isDanger;
        }
        return hasDanger;
    }
    $scope.queryParams = { queries: {}, order: [], baseUrl: '' };
    function setIFrame(data) {
        $scope.iFrameData = $sce.trustAsResourceUrl('data:text/html;charset=utf-8,' + encodeURI(data));
    }
    function setStatus(text) {
        $scope.statusText = text;
    }
    function applyFilter() {
        var filterText = $scope.filterText;
        localStorage.setItem('filterText', filterText);
        var arr = $scope.page.HtmlErrors;
        if (filterText != null && filterText.length > 0) {
            arr = filter(arr, filterText);
            setStatus('Filtered ' + +' Elements');
        }
        else {
            setStatus('');
        }
        mapHtmlErrors(arr);
        $scope.htmlErrors = arr;
    }
    function filter(htmlErrors, text) {
        var arr = [];
        for (var x = 0; x < htmlErrors.length; x++) {
            var e = htmlErrors[x];
            if (e.TagName.indexOf(text) > -1 || e.AttributeName.indexOf(text) > -1) {
                arr.push(e);
            }
        }
        return arr;
    }
    function getQueryParams(url) {
        var queryp = { queries: {}, order: [], baseUrl: '', fullurl: '' };
        if (url == null)
            return queryp;
        queryp.fullurl = url;
        var urlSplitted = url.split('?');
        queryp.baseUrl = urlSplitted[0];
        if (urlSplitted.length <= 1)
            return queryp;
        var queries = urlSplitted[1].split('&');
        for (var x = 0; x < queries.length; x++) {
            var q = queries[x];
            var kv = q.split('=');
            queryp.queries[kv[0]] = kv[1];
            queryp.order[x] = kv[0];
        }
        return queryp;
    }
    function buildUrlFromQueryParams(q) {
        var url = $scope.inputUrl;
        var baseUrl = url.split('?')[0];
        var query = '';
        for (var x = 0; x < q.order.length; x++) {
            query += q.order[x] + '=' + q.queries[q.order[x]];
            if (x < q.order.length - 1)
                query += '&';
        }
        return baseUrl + '?' + query;
    }
    function mapHtmlErrors(arr) {
        for (var x = 0; x < arr.length; x++) {
            var v = arr[x];
            if (v.Reason == 0)
                v.reasonText = 'invalid tag';
            else if (v.Reason == 1)
                v.reasonText = 'invalid attribute';
            else if (v.Reason == 2)
                v.reasonText = 'closed before opened';
            else if (v.Reason == 3)
                v.reasonText = 'not closed';
            else if (v.Reason == 4)
                v.reasonText = 'duplicate attribute';
        }
    }
    $scope.updateUrlFromQuery = function (q) {
        $scope.inputUrl = buildUrlFromQueryParams(q);
    };
    $scope.$watch('inputUrl', function (nVal, oVal) {
        $scope.queryParams = getQueryParams(nVal);
    });
    $scope.$watch('scanner.scanVectors', function (nVal, oVal) {
        $scope.scanner.scanVectorsArr = [];
        var vectors = nVal.split('\n');
        for (var x = 0; x < vectors.length; x++) {
            $scope.scanner.scanVectorsArr.push(vectors[x]);
        }
    });
    $scope.updateQueryParams = function (qp, name, val) {
        if ($scope.oldUrlQuery == null)
            $scope.oldUrlQuery = angular.copy(qp);
        qp.queries[name] = val;
        $scope.updateUrlFromQuery(qp);
    };
    $scope.restoreQueryParams = function (qp, name) {
        if ($scope.oldUrlQuery == null)
            return;
        $scope.updateUrlFromQuery($scope.oldUrlQuery);
        $scope.oldUrlQuery = null;
    };
    $scope.addTag = function (htmlError) {
        var tagName = htmlError.TagName;
        var attrName = htmlError.AttributeName;
        $http({
            method: 'POST',
            url: '/api/addTag',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            data: { TagName: tagName, AttributeName: attrName },
            transformRequest: function (obj) {
                var str = [];
                for (var p in obj)
                    str.push(encodeURIComponent(p) + "=" + encodeURIComponent(obj[p]));
                return str.join("&");
            }
        }).then(function () {
            removeElement($scope.htmlErrors, htmlError);
        });
    };
    function removeElement(arr, el) {
        for (var x = 0; x < arr.length; x++) {
            if (arr[x] == el) {
                arr.splice(x, 1);
                return;
            }
        }
    }
    function saveUserSettings() {
        localStorage.setItem("crawlurl", $scope.inputUrl);
        localStorage.setItem("encodeUrl", $scope.encodeUrl);
        localStorage.setItem("userScript", $scope.userScript);
        localStorage.setItem("dangerTags", $scope.htmlValidator.dangerTags);
        localStorage.setItem("dangerAttributes", $scope.htmlValidator.dangerAttributes);
        localStorage.setItem("scanVectors", $scope.scanner.scanVectors);
        localStorage.setItem("filterText", $scope.filterText);
    }
    function loadUserSettings() {
        var url = localStorage.getItem("crawlurl");
        var encodeUrl = localStorage.getItem("encodeUrl");
        var userScript = localStorage.getItem("userScript");
        var filterText = localStorage.getItem("filterText");
        $scope.htmlValidator.dangerTags = localStorage.getItem("dangerTags");
        $scope.htmlValidator.dangerAttributes = localStorage.getItem("dangerAttributes");
        $scope.scanner.scanVectors = localStorage.getItem("scanVectors");
        if (url != null)
            $scope.inputUrl = url;
        if (encodeUrl != null)
            $scope.encodeUrl = encodeUrl == 'true' ? true : false;
        if (userScript != null)
            $scope.userScript = userScript;
        if (filterText != null)
            $scope.filterText = filterText;
    }
});
