package ggrok

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

type GGrokClient struct {
}

func NewClient() *GGrokClient {
	return &GGrokClient{}
}

func (ggclient *GGrokClient) Start() {

	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/$$ggrok"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read message error:", err)
				continue
			}
			log.Printf("recv: %s", message)

			var websocketReq WebSocketRequest
			if err := json.Unmarshal(message, &websocketReq); err != nil {
				log.Println("json.Unmarshal error", err)
				continue
			}

			var localRequest *http.Request
			r := bufio.NewReader(bytes.NewReader([]byte(websocketReq.Req)))
			if localRequest, err = http.ReadRequest(r); err != nil { // deserialize request
				log.Println("deserialize request error", err)
				continue
			}

			//TODO: change to config
			localRequest.RequestURI = ""
			u, err := url.Parse("/ada08e16-2112-4720-8fcb-18f2f8e47c2d")
			if err != nil {
				log.Println("parse url error", err)
			}
			localRequest.URL = u
			localRequest.URL.Scheme = "https"
			localRequest.URL.Host = "webhook.site"
			resp, err := (&http.Client{}).Do(localRequest)
			if err != nil {
				log.Println("local http request error:", err)
				continue
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Println("read local response error ", err)
			}
			resp.Body.Close()
			wsRes := WebSocketResponse{Status: resp.Status, StatusCode: resp.StatusCode,
				Proto: resp.Proto, Header: resp.Header, Body: body, ContentType: resp.Header.Get("Content-Type")}

			log.Printf("client send response: %s \n", wsRes.Body)
			c.WriteJSON(wsRes)
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}