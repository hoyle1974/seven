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
	_ "embed"

	"flag"
	"net/http"
	"os"
	"text/template"

	ginzerolog "github.com/dn365/gin-zerolog"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hellofresh/health-go/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

//go:embed index.html
var indexHtml string
var homeTemplate = template.Must(template.New("").Parse(indexHtml))

//go:embed client.js
var clientJS string
var clientTemplate = template.Must(template.New("").Parse(clientJS))

var addr = flag.String("addr", ":8080", "http service address")
var debug = flag.Bool("debug", true, "Enable debug")

var upgrader = websocket.Upgrader{} // use default option

type EntryForm struct {
	Uuid    string `form:"uuid" json:"uuid" binding:"required"`
	Address string `form:"addr" json:"addr" binding:"required"`
}

func register(ctx *gin.Context) {
	var json EntryForm
	err := ctx.BindJSON(&json)
	if err != nil {
		log.Err(err).Msg("Error parsing form")
		ctx.JSON(http.StatusNotAcceptable, gin.H{"status": "error parsing json"})
		return
	}

	entries, err := registerJSON(json)
	if err != nil {
		log.Err(err).Msg("Error converting uuid string to actual uuid")
		ctx.JSON(http.StatusNotAcceptable, gin.H{"status": "not acceptable"})

	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "entries": entries})
}

func registerWS(ctx *gin.Context) {
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

/*
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
*/

func home(c *gin.Context) {
	homeTemplate.Execute(c.Writer, "ws://"+c.Request.Host+"/ws/register")
}

func client(c *gin.Context) {
	clientTemplate.Execute(c.Writer, "ws://"+c.Request.Host+"/ws/register")
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

	// r.GET("/echo", echo)
	r.GET("/", home)
	r.GET("/client.js", client)
	r.GET("/ws/register", registerWS)
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
