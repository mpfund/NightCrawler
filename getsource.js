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
        window.document.write = function(k){
        	window.document.__writes.push(k);
        	window.document.__write(k);
        }
        window.document.__writeln = document.writeln;
        window.document.writeln = function(k){
        	window.document.__writes.push(k);
        	window.document.__writeln(k);
        }
    });
};
page.onError = function(){}

page.open(args[1], function(status) {
	var writes = page.evaluate(function(){
		return window.document.__writes;
	})
    if(status=='success'){
  var ret = {Body:page.content,'JSwrites':writes};
  console.log(JSON.stringify(ret));
}else{console.log({Body:'error','JSwrites':[]})}
  phantom.exit();
});