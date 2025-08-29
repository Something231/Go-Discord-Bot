package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
)

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

	var heartbeat struct {
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

			err = json.Unmarshal(message, &heartbeat)
			if err != nil {
				fmt.Println("Something has gone bad bad very bad", err)
			} else if heartbeat.OP == 10 {
				fmt.Println("Hello event processed")
				return
			}
		}
	}()
	fmt.Println("Exiting")

	//this is slightly inspired from gorilla/websocket example code
	done := make(chan struct{})
	defer close(done)
}
