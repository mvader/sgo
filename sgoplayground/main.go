package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/tcard/sgo/sgo"
	"github.com/tcard/sgo/sgo/format"
	"github.com/tcard/sgo/sgo/scanner"
)

var (
	httpAddr = flag.String("http", ":5600", "HTTP server address")

	upgrader = websocket.Upgrader{}
)

func main() {
	flag.Parse()

	msgCh := make(chan msgType)
	go func() {
		for msg := range msgCh {
			switch msg.Type {
			case "format":
				resp := &msgType{
					Type: "format",
				}
				func() {
					defer func() {
						if r := recover(); r != nil {
							value := fmt.Sprintln(r)
							stack := make([]byte, 99999)
							runtime.Stack(stack, false)
							value += string(stack)
							resp.Value = value
						}
					}()
					formatted, err := format.Source([]byte(msg.Value.(string)))
					if err == nil {
						resp.Value = string(formatted)
					}
				}()
				msg.c.WriteJSON(resp)
			case "translate":
				resp := &msgType{
					Type: "translate",
				}
				func() {
					defer func() {
						if r := recover(); r != nil {
							value := fmt.Sprintln(r)
							stack := make([]byte, 99999)
							runtime.Stack(stack, false)
							value += string(stack)
							resp.Value = value
						}
					}()
					w := &bytes.Buffer{}
					err := sgo.TranslateFile(w, strings.NewReader(msg.Value.(string)), "name")
					if err != nil {
						if errs, ok := err.(scanner.ErrorList); ok {
							var errMsgs []string
							for _, err := range errs {
								errMsgs = append(errMsgs, err.Error())
							}
							resp.Value = strings.Join(errMsgs, "\n")
						} else {
							resp.Value = err.Error()
						}
					} else {
						resp.Value = w.String()
					}
				}()
				msg.c.WriteJSON(resp)
			case "execute":
				resp := &msgType{
					Type: "execute",
				}
				body := url.Values{}
				body.Add("version", "2")
				var err error
				w := &bytes.Buffer{}
				func() {
					defer func() {
						if r := recover(); r != nil {
							value := fmt.Sprintln(r)
							stack := make([]byte, 99999)
							runtime.Stack(stack, false)
							value += string(stack)
							err = errors.New(value)
						}
					}()

					err = sgo.TranslateFile(w, strings.NewReader(msg.Value.(string)), "name")
				}()
				if err != nil {
					resp.Value = err.Error()
				} else {
					body.Add("body", w.String())
					postResp, err := http.PostForm("http://play.golang.org/compile", body)
					if err != nil {
						resp.Value = err.Error()
					} else {
						var v interface{}
						err := json.NewDecoder(postResp.Body).Decode(&v)
						postResp.Body.Close()
						if err != nil {
							resp.Value = err.Error()
						} else {
							resp.Value = v
						}
					}
				}
				msg.c.WriteJSON(resp)
			}
		}
	}()

	http.HandleFunc("/ws", func(w http.ResponseWriter, req *http.Request) {
		c, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		defer c.Close()
		for {
			var recvMsg msgType
			err := c.ReadJSON(&recvMsg)
			if err != nil {
				log.Println("read:", err)
				break
			}
			recvMsg.c = c

			msgCh <- recvMsg
		}

	})

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		gist := req.URL.Query().Get("gist")
		preloadedCode := ""
		if gist == "" {
			preloadedCode = defaultPreloadedCode
		}
		indexTpl.Execute(w, map[string]interface{}{
			"Gist":          gist,
			"WSURL":         "ws://" + req.Host + "/ws",
			"PreloadedCode": preloadedCode,
		})
	})

	fmt.Println("Serving on", *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}

type msgType struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
	c     *websocket.Conn
}

const defaultPreloadedCode = `package main

import (
	"errors"
	"fmt"
)

type Result struct {
	a int
}

func Foo(i int) (*Result, int \ error) {
	if i%2 == 0 {
		return &Result{i}, i * 2 \
	}
	// return nil, errors.New("hola") // doesn't compile
	// return nil, nil		 // doesn't compile
	return  \ errors.New("hola")
}

func main() {
	res, i \ err := Foo(123)
	// _ = err.Error() // doesn't compile
	if err == nil {
		// _ = err.Error() // doesn't compile
		fmt.Println("Result:", res, "i:", i) // will never panic; res will never be nil
	} else {
		fmt.Println("Error:", err.Error())
		// fmt.Println(res, i, err) // doesn't compile; res is entangled here
	}
	// fmt.Println(res, i, err) // doesn't compile; res is entangled here
}`

var indexTpl = template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html lang="en">

<head>
  <meta charset="utf-8">
  <title>SGo playground</title>
</head>

<body>

<div style="width: 50%; float: left;">
<textarea id="input-code" style="width: 90%;" rows="30">
{{.PreloadedCode}}
</textarea>
</div>

<div>
<pre id="translated" style="height: 100%; max-height: 390px; overflow: scroll;">
</pre>
</div>

<div style="clear: both;">
  <button id="run-button">Run</button>
  <button id="format-button">Format</button>
  <button id="share-button">Share</button>
  <input type="text" id="share-input" style="display: none;">

  <div>
  <pre id="executed">
  </pre>
  </div>

</div>

<script>
window.addEventListener("load", function(evt) {
	var inputCode = document.getElementById("input-code");
	var translated = document.getElementById("translated");
	var runButton = document.getElementById("run-button");
	var formatButton = document.getElementById("format-button");
	var shareButton = document.getElementById("share-button");
	var shareInput = document.getElementById("share-input");
	var executed = document.getElementById("executed");

	var ws = new WebSocket("{{.WSURL}}");
	ws.onmessage = function(ev) {
		var data = JSON.parse(ev.data);
		if (data.type == "execute") {
			if (data.value.Events) {
				var evs = data.value.Events;
				var accDelay = 0;
				for (var i in evs) {
				(function(i) {
					var ev = evs[i];
					accDelay += ev.Delay;
					setTimeout(function() {
						executed.textContent += ev.Message;
						if (i == evs.length - 1) {
							runButton.textContent = "Run";
							runButton.disabled = false;
						}
					}, accDelay);
				})(i);
				}
			} else if (data.value.Errors) {
				executed.textContent = data.value.Errors;
				runButton.textContent = "Run";
				runButton.disabled = false;
			} else {
				runButton.textContent = "Run";
				runButton.disabled = false;
			}
		} else if (data.type == "translate") {
			translated.textContent = data.value;
		} else if (data.type == "format") {
			if (data.value) {
				inputCode.value = data.value;
			}
			formatButton.textContent = "Format";
			formatButton.disabled = false;
		}
	};

	runButton.onclick = function(ev) {
		ev.preventDefault();
		ws.send(JSON.stringify({
			"type": "execute",
			"value": inputCode.value,
		}));
		runButton.textContent = "Running...";
		runButton.disabled = true;
		executed.textContent = "";
	};

	formatButton.onclick = function(ev) {
		ev.preventDefault();
		ws.send(JSON.stringify({
			"type": "format",
			"value": inputCode.value,
		}));
		formatButton.textContent = "Formatting...";
		formatButton.disabled = true;
	};

	var translate = function() {
		ws.send(JSON.stringify({
			"type": "translate",
			"value": inputCode.value,
		}));
	};

	inputCode.onchange = translate;
	inputCode.onkeyup = translate;
	ws.onopen = function() {
		var gist = "{{.Gist}}";
		if (gist) {
			runButton.textContent = "Loading Gist...";
			runButton.disabled = true;
			ajax("https://api.github.com/gists/" + gist, {
				success: function(req) {
					var files = JSON.parse(req.responseText).files;
					for (var i in files) {
						ajax(files[i].raw_url, {success: function(req) {
							inputCode.value = req.responseText;
							translate();
							runButton.textContent = "Run";
							runButton.disabled = false;
						}});
						break;
					}
				},
				fail: function(req) {
					alert("Bad Gist URL!");
					runButton.textContent = "Run";
					runButton.disabled = false;
				},
			});
		} else {
			translate();
		}
	};

	shareButton.onclick = function(ev) {
		ev.preventDefault();
		ajax("https://api.github.com/gists", {
			method: 'POST',
			data: JSON.stringify({
				files: {
					'sgoplayground.go': {
						content: inputCode.value
					}
				}
			}),
			success: function(req) {
				var id = JSON.parse(req.responseText).id;
				shareInput.value = window.location.origin + window.location.pathname + '?gist=' + id;
				shareInput.style.display = 'inline';
				shareInput.focus();
				shareButton.textContent = "Share";
				shareButton.disabled = false;
			},
		});
		shareButton.textContent = "Sharing...";
		shareButton.disabled = true;
	};

	// From http://stackoverflow.com/a/18303822/818420
	inputCode.addEventListener('keydown',function(e) {
		if(e.keyCode === 9) { // tab was pressed
			// get caret position/selection
			var start = this.selectionStart;
			var end = this.selectionEnd;

			var target = e.target;
			var value = target.value;

			// set textarea value to: text before caret + tab + text after caret
			target.value = value.substring(0, start)
						+ "\t"
						+ value.substring(end);

			// put caret at right position again (add one for the tab)
			this.selectionStart = this.selectionEnd = start + 1;

			// prevent the focus lose
			e.preventDefault();
		}
	},false);
});

// From http://www.debrice.com/micro-ajax-library/
function ajax(e,t,i){i=window.XMLHttpRequest?new XMLHttpRequest:new ActiveXObject("Microsoft.XMLHTTP"),i.onreadystatechange=function(){4==i.readyState&&(/^2/.test(i.status)&&t.success?t.success(i):/^5/.test(i.status)&&t.fail?t.fail(i):t.other&&t.other(i))},i.open(t.method||"GET",e,!0),i.send(t.data)}
</script>
</body>

</html>
`))
