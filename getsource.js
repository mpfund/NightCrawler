var system = require('system');
var args = system.args;
var page = require('webpage').create();

/*
page.onConsoleMessage = function (msg) {
    console.log('From Page Console: '+msg);
};*/
page.onInitialized = function () {
    page.evaluate(function () {
        window.document.__write = document.write;
        window.document.__writes = [];
        window.__evals = [];
        window.__timeouts = [];
        window.document.write = function(k){
        	window.document.__writes.push(k);
        	window.document.__write(k);
        }
        window.document.__writeln = document.writeln;
        window.document.writeln = function(k){
        	window.document.__writes.push(k);
        	window.document.__writeln(k);
        }
        window.__eval = window.eval;
        window.eval = function(k){
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

function totext(k){
    if(k===null)
        return null;
    return k.toString();
}
page.onError = function(){}

page.open(args[1], function(status) {
	var writes = page.evaluate(function(){
		return window.document.__writes;
	})
    var evals = page.evaluate(function(){
        return window.__evals;
    })
    var timeouts = page.evaluate(function(){
        return window.__timeouts;
    })

    if(status=='success'){
  var ret = {Body:page.content,'JSwrites':writes,
  'JSevals':evals,'JStimeouts':[],'Cookies':page.cookies};
  console.log(JSON.stringify(ret));
}else{console.log({Body:'error','JSwrites':[]})}
  phantom.exit();
});