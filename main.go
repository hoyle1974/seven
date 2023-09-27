// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Hello is a simple hello, world demonstration web server.
//
// It serves version information on /version and answers
// any other request like /name by saying "Hello, name!".
//
// See golang.org/x/example/outyet for a more sophisticated server.
package main

import (
	"flag"
	"math/rand"
	"net/http"
	"os"
	"text/template"
	"time"

	ginzerolog "github.com/dn365/gin-zerolog"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hellofresh/health-go/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var addr = flag.String("addr", ":8080", "http service address")
var debug = flag.Bool("debug", true, "Enable debug")

var upgrader = websocket.Upgrader{} // use default option

var cache, _ = lru.New[string, Entry](1024)

type Entry struct {
	uuid     uuid.UUID
	address  string
	lastSeen time.Time
}

func (e Entry) ToEntryJson() EntryForm {
	return EntryForm{
		Uuid:    e.uuid.String(),
		Address: e.address,
	}
}

type EntryForm struct {
	Uuid    string `form:"uuid" json:"uuid" binding:"required"`
	Address string `form:"addr" json:"addr" binding:"required"`
}

func genGoodRandom(max int, bad map[int]bool) int {
	n := -1
	maxTries := 5
	for maxTries > 0 {
		p := rand.Intn(max)
		if !bad[p] {
			n = p
			break
		}
		maxTries--
	}
	return n
}

func pickSome(values []Entry, amount int) []EntryForm {
	picked := make([]EntryForm, 0)
	bad := make(map[int]bool, 0)

	if len(values) == 0 {
		return picked
	}

	for amount > 0 {
		idx := genGoodRandom(len(values), bad)
		if idx == -1 {
			break
		}
		bad[idx] = true
		picked = append(picked, values[idx].ToEntryJson())
		amount--
	}

	return picked
}

func register(ctx *gin.Context) {
	var json EntryForm
	err := ctx.BindJSON(&json)
	if err != nil {
		log.Err(err).Msg("Error parsing form")
		ctx.JSON(http.StatusNotAcceptable, gin.H{"status": "error parsing json"})
		return
	}

	// Extract and validate uuid
	uuid, err := uuid.Parse(json.Uuid)
	if err != nil {
		log.Err(err).Msg("Error converting uuid string to actual uuid")
		ctx.JSON(http.StatusNotAcceptable, gin.H{"status": "not acceptable"})
		return
	}
	if len(json.Address) < 1 {
		log.Err(err).Msg("Address was empty")
		ctx.JSON(http.StatusNotAcceptable, gin.H{"status": "not acceptable"})
		return
	}

	entries := pickSome(cache.Values(), 16)

	entry := Entry{
		uuid:     uuid,
		address:  json.Address,
		lastSeen: time.Now(),
	}

	// Store this uuid and it's address
	log.Debug().Str("uuid", json.Uuid).Msg("Registering client")
	cache.Add(json.Uuid, entry)

	// TODO - get a set of address to return
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "entries": entries})
}

func echo(ctx *gin.Context) {
	w, r := ctx.Writer, ctx.Request
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().AnErr("upgrade", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Error().AnErr("read", err)
			break
		}
		log.Printf("recv:%s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Error().AnErr("write", err)
			break
		}
	}
}

func home(c *gin.Context) {
	homeTemplate.Execute(c.Writer, "ws://"+c.Request.Host+"/echo")
}

func main() {
	flag.Parse()
	if !*debug {
		gin.SetMode(gin.ReleaseMode)
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Info().Msg("Seven - a WebRTC signaling server")

	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		log.Debug().Str("httpMethod", httpMethod).Str("absolutePath", absolutePath).Str("handlerName", handlerName).Int("nuHandlers", nuHandlers)
	}

	r := gin.New()
	r.Use(ginzerolog.Logger("gin"))
	r.Use(gin.Recovery())
	r.SetTrustedProxies(nil)

	r.GET("/echo", echo)
	r.GET("/", home)
	r.POST("/register", register)

	h, _ := health.New(
		health.WithSystemInfo(),
		health.WithComponent(health.Component{
			Name:    "Seven",
			Version: "v0.1",
		}))
	r.GET("/health", func(ctx *gin.Context) {
		w, r := ctx.Writer, ctx.Request
		h.HandlerFunc(w, r)
	})

	log.Fatal().AnErr("Run", r.Run(*addr))
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>  
window.addEventListener("load", function(evt) {
    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;
    var print = function(message) {
        var d = document.createElement("div");
        d.textContent = message;
        output.appendChild(d);
        output.scroll(0, output.scrollHeight);
    };
    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("{{.}}");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print("RESPONSE: " + evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };
    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };
    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    };
});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server, 
"Send" to send a message to the server and "Close" to close the connection. 
You can change the message and send multiple times.
<p>
<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><input id="input" type="text" value="Hello world!">
<button id="send">Send</button>
</form>
</td><td valign="top" width="50%">
<div id="output" style="max-height: 70vh;overflow-y: scroll;"></div>
</td></tr></table>
</body>
</html>
`))
