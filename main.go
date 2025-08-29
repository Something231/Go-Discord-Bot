package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
)

func main() {
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
	connection.Close()
	if err != nil {
		fmt.Println("Failed to connect to gateway :(")
		return
	}
	fmt.Println("Connected to discord gateway")
}
