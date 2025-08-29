package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// var lastsequencenumber int
var heartbeatrecieved int32
var sequencenumber int64

func main() {
	defer fmt.Println("Program exited")
	response, err := http.Get("https://discord.com/api/gateway")
	if err != nil {
		fmt.Println("Request Failed")
		return
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)

	if response.StatusCode > 299 {
		fmt.Printf("Response failed with status code: %d and\nbody: %s\n", response.StatusCode, body)
	}
	if err != nil {
		fmt.Println(err)
		return
	}

	var gateway struct {
		URL string `json:"url"`
	}
	err = json.Unmarshal(body, &gateway)

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(gateway.URL)

	connection, response, err := websocket.DefaultDialer.Dial(gateway.URL+"/?v=10&encoding=json", nil)

	if err != nil {
		fmt.Println("Failed to connect to gateway :(")
		return
	}
	defer connection.Close()

	fmt.Println("Connected to discord gateway")

	var hello struct {
		OP int `json:"op"`
		D  struct {
			HeartbeatInterval int `json:"heartbeat_interval"`
		} `json:"d"`
	}

	func() {
		for {
			_, message, err := connection.ReadMessage()
			if err != nil {
				fmt.Println("read:", err)
			}

			err = json.Unmarshal(message, &hello)
			if err != nil {
				fmt.Println("Something has gone bad bad very bad", err)
			} else if hello.OP == 10 {
				fmt.Println("Hello event processed")
				return
			}
		}
	}()

	fmt.Println(hello.D.HeartbeatInterval)
	go func() {
		time.Sleep(time.Duration(float64(hello.D.HeartbeatInterval)*rand.Float64()) * time.Millisecond)
		fmt.Println("heartbeat")
		heartbeaterr := connection.WriteMessage(websocket.TextMessage, []byte(`{"op":1,"d":null}`))
		if heartbeaterr != nil {
			fmt.Println("Technically this is handling the error")
		}

		for {
			time.Sleep(time.Duration(float64(hello.D.HeartbeatInterval)) * time.Millisecond)

			if atomic.LoadInt32(&heartbeatrecieved) == 0 {
				println("At this point, this is where you know its over.")
				return
			}

			var heartbeaterr error
			fmt.Println("sending heartbeat")
			if atomic.LoadInt64(&sequencenumber) == 0 {
				heartbeaterr = connection.WriteMessage(websocket.TextMessage, []byte(`{"op":1,"d":null}`))
			} else {
				heartbeaterr = connection.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"op":1,"d":%d}`, atomic.LoadInt64(&sequencenumber))))
			}
			if heartbeaterr != nil {
				fmt.Println("Technically this is handling the error")
			}
			atomic.StoreInt32(&heartbeatrecieved, 0)
		}
	}()

	// Reading loop below:
	go func() {
		for {
			var event struct {
				OP int    `json:"op"`
				D  any    `json:"d"`
				S  int    `json:"s"`
				T  string `json:"t"`
			}
			readerr := connection.ReadJSON(&event)
			if readerr != nil {
				fmt.Println("Idk not my problem (actually it is my problem)", readerr)
				return
			}

			switch event.OP {
			case 11:
				atomic.StoreInt32(&heartbeatrecieved, 1)
				fmt.Println("heartbeat recieved")
			case 1:
				fmt.Println("Spanish Inquisition apparently")
				var heartbeaterr error
				if atomic.LoadInt64(&sequencenumber) == 0 {
					heartbeaterr = connection.WriteMessage(websocket.TextMessage, []byte(`{"op":1,"d":null}`))
				} else {
					heartbeaterr = connection.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"op":1,"d":%d}`, atomic.LoadInt64(&sequencenumber))))
				}

				if heartbeaterr != nil {
					fmt.Println("Technically this is handling the error")
				}
			case 0:
				print(event.OP, event.D, event.S, event.T)
			}
			fmt.Println("a", event.OP)

			if event.S != 0 {
				atomic.StoreInt64(&sequencenumber, int64(event.S))
			}

		}
	}()

	for {
		time.Sleep(500)
	}
}
